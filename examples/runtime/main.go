// Runtime demonstrates an application composed from gx Runtime containers.
//
// Run with:
//
//	go run ./examples/runtime
//
// Press Ctrl-C (or send SIGTERM) to begin graceful shutdown.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/shayanderson/gx"
)

// Config is a Runtime-global dependency. Every container can retrieve it with
// Container.Global.
type Config struct {
	Name string
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	rt := gx.NewRuntime()

	// Set registers a dependency before initialization begins. Runtime-global
	// dependencies are shared by every container.
	rt.Set(Config{Name: "gx runtime example"})

	// Create attaches a container and runs its factory during initialization.
	// A successful factory result is registered as a Runtime-global dependency,
	// so containers attached later can retrieve *Server with c.Global[*Server]().
	rt.Create("server", newServer)

	// Attach creates a container for services that only need local state. The
	// Runtime initializes and starts containers in attachment order, then stops
	// them in reverse order.
	attachWorker(rt)

	if err := rt.Run(context.Background(), gx.RunOptions{
		// Logger receives gx lifecycle and managed-task logs. A nil logger
		// disables those logs.
		Logger: logger,

		// NoSignals leaves Runtime-managed signal handling enabled. Set this to
		// true when the caller, rather than the Runtime, owns cancellation;
		// Signals is then ignored.
		NoSignals: false,

		// Signals replaces the default os.Interrupt and syscall.SIGTERM shutdown
		// signal set.
		Signals: []os.Signal{os.Interrupt, syscall.SIGTERM},

		// ShutdownTimeout bounds each OnStopping and OnStop hook. It does not
		// force managed tasks to stop: Runtime keeps waiting and logs outstanding
		// tasks at this interval until they exit.
		ShutdownTimeout: 10 * time.Second,
	}); err != nil {
		slog.Error("runtime failed", slog.String("err", err.Error()))
	}
}

// Server is a Runtime-global service created by the "server" container.
type Server struct {
	ready     chan struct{}
	stop      chan struct{}
	readyOnce sync.Once
	stopOnce  sync.Once
}

// newServer builds a Runtime-global service. Most containers follow this shape:
// register their managed tasks directly, without needing an OnStart hook.
func newServer(c *gx.Container) (*Server, error) {
	server := &Server{
		ready: make(chan struct{}),
		stop:  make(chan struct{}),
	}

	// Run is normally registered during construction or OnInit. This task starts
	// after this container's OnStart hooks succeed; because this container has no
	// startup hooks, it starts as soon as the server container starts.
	c.Run("serve", server.run)

	// OnStopping runs before Runtime waits for this container's managed tasks.
	// It is the place to stop accepting new work or otherwise unblock a task.
	c.OnStopping("stop accepting requests", func(_ context.Context, _ *gx.Container) error {
		server.stopOnce.Do(func() { close(server.stop) })
		slog.Info("server is stopping")
		return nil
	})

	// OnStop runs only after this container's managed tasks have exited. Use it
	// to finalize resources that those tasks might still have been using.
	c.OnStop("close resources", server.close)

	return server, nil
}

func (s *Server) run(ctx context.Context) error {
	// A real server would close ready after it begins accepting connections.
	s.readyOnce.Do(func() { close(s.ready) })

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.stop:
		return nil
	}
}

func (s *Server) waitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) close(_ context.Context, _ *gx.Container) error {
	slog.Info("server resources closed")
	return nil
}

// Worker is container-local state: it is available only in the "worker"
// container with Container.Get.
type Worker struct {
	message string
}

func attachWorker(rt *gx.Runtime) {
	c := rt.Attach("worker")

	// OnInit runs before any container's OnStart hook. It can retrieve global
	// dependencies, construct local state, register shutdown hooks, and queue
	// tasks. Queued tasks begin only after this container's OnStart hooks succeed.
	c.OnInit("build worker", func(c *gx.Container) error {
		cfg := c.Global[Config]()
		server := c.Global[*Server]()
		worker := &Worker{message: fmt.Sprintf("hello from %s", cfg.Name)}
		c.Set(worker)

		// This task is queued during initialization. It starts after worker
		// startup succeeds.
		c.Run("greeting", func(ctx context.Context) error {
			return worker.run(ctx, server)
		})
		return nil
	})

	// OnStart is optional. Use it when synchronous setup must finish before this
	// container's queued tasks start. It does not need to register tasks itself.
	c.OnStart("verify server", func(c *gx.Container) error {
		server := c.Global[*Server]()
		cfg := c.Global[Config]()
		if err := server.waitReady(c.Ctx()); err != nil {
			return err
		}
		slog.Info("worker startup complete", slog.String("application", cfg.Name))
		return nil
	})

	// Stop hooks run in reverse registration order after managed tasks exit.
	c.OnStop("finalize worker", func(_ context.Context, c *gx.Container) error {
		return c.Get[*Worker]().finalize()
	})
}

func (w *Worker) run(ctx context.Context, server *Server) error {
	select {
	case <-server.ready:
		slog.Info("worker running", slog.String("message", w.message))
	case <-ctx.Done():
		return ctx.Err()
	}

	<-ctx.Done()
	return ctx.Err()
}

func (w *Worker) finalize() error {
	slog.Info("worker finalized")
	return nil
}
