package gx

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/shayanderson/gx/test"
)

type createdDependency struct {
	value string
}

type lockedBuffer struct {
	b  bytes.Buffer
	mu sync.Mutex
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.b.String()
}

func (b *lockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.b.Write(value)
}

func TestRunOptions(t *testing.T) {
	t.Parallel()

	defaults := runOptions(nil)
	test.Nil(t, defaults.Logger)
	test.False(t, defaults.NoSignals)
	test.Len(t, 0, defaults.Signals)
	test.Equal(t, runtimeDefaultShutdownTimeout, defaults.ShutdownTimeout)

	logger := slog.Default()
	signals := []os.Signal{os.Interrupt}
	options := runOptions([]RunOptions{{
		Logger:          logger,
		NoSignals:       true,
		Signals:         signals,
		ShutdownTimeout: time.Second,
	}})

	test.Equal(t, logger, options.Logger)
	test.True(t, options.NoSignals)
	test.Equal(t, signals, options.Signals)
	test.Equal(t, time.Second, options.ShutdownTimeout)

	options = runOptions([]RunOptions{{ShutdownTimeout: -time.Second}})
	test.Equal(t, runtimeDefaultShutdownTimeout, options.ShutdownTimeout)
}

func TestRuntimeStore(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	test.Equal(t, rt, rt.Set("runtime value"))
	test.Equal(t, "runtime value", rt.Get[string]())

	value, ok := rt.GetTry[string]()
	test.True(t, ok)
	test.Equal(t, "runtime value", value)

	_, ok = rt.GetTry[int]()
	test.False(t, ok)

	c := rt.Attach("service")
	test.Equal(t, c, c.Set(42))
	test.Equal(t, 42, c.Get[int]())
	test.Equal(t, "runtime value", c.Global[string]())
	test.Equal(t, c, c.SetGlobal(1).SetGlobal(true))
	test.Equal(t, 1, c.Global[int]())
	test.True(t, c.Global[bool]())

	value, ok = c.GlobalTry[string]()
	test.True(t, ok)
	test.Equal(t, "runtime value", value)

	value, ok = c.GetTry[string]()
	test.False(t, ok)
	test.Empty(t, value)

	count, ok := c.GetTry[int]()
	test.True(t, ok)
	test.Equal(t, 42, count)
}

func TestRuntimeStorePanics(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()

	test.Panics(t, func() {
		rt.Get[string]()
	})

	rt.Set(1)

	test.Panics(t, func() {
		rt.Set(2)
	})

	c := rt.Attach("service")

	test.Panics(t, func() {
		c.Get[string]()
	})
	test.Panics(t, func() {
		c.Global[string]()
	})

	test.Panics(t, func() {
		rt.Attach("service")
	})
}

func TestRuntimeAttach(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	c := rt.Attach("service")

	test.Equal(t, "service", c.Name())

	test.Panics(t, func() {
		rt.Attach("")
	})
}

func TestRuntimeCreate(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	var events []string
	add := func(event string) {
		events = append(events, event)
	}

	test.Equal(t, rt, rt.Create("first", func(c *Container) (*createdDependency, error) {
		add("first:create")
		test.True(t, c.Ctx() != nil)
		c.OnStart("start", func(*Container) error {
			add("first:start")
			return nil
		})
		c.OnStopping("stop", func(context.Context, *Container) error {
			add("first:stop")
			return nil
		})
		return &createdDependency{value: "created"}, nil
	}))
	rt.Create("second", func(c *Container) (int, error) {
		add("second:create")
		test.Equal(t, "created", c.Global[*createdDependency]().value)
		return 42, nil
	})

	test.NoError(t, rt.Run(t.Context(), RunOptions{NoSignals: true}))
	test.Equal(t, "created", rt.Get[*createdDependency]().value)
	test.Equal(t, 42, rt.Get[int]())
	test.Equal(t, []string{
		"first:create",
		"second:create",
		"first:start",
		"first:stop",
	}, events)
}

func TestRuntimeCreateError(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	errCreate := errors.New("create failed")

	var started bool
	var stopped bool
	rt.Create("first", func(c *Container) (int, error) {
		c.OnStart("start", func(*Container) error {
			started = true
			return nil
		})
		c.OnStopping("stop", func(context.Context, *Container) error {
			stopped = true
			return nil
		})
		return 42, nil
	})

	rt.Create("failing", func(*Container) (string, error) {
		return "", errCreate
	})

	var nextCreated bool
	rt.Create("next", func(*Container) (bool, error) {
		nextCreated = true
		return true, nil
	})

	err := rt.Run(t.Context(), RunOptions{NoSignals: true})

	test.ErrorIs(t, err, errCreate)
	test.Equal(t, 42, rt.Get[int]())
	test.False(t, started)
	test.True(t, stopped)
	test.False(t, nextCreated)
}

func TestRuntimeSetupPanicsAfterRun(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	c := rt.Attach("service")
	test.NoError(t, rt.Run(t.Context(), RunOptions{NoSignals: true}))

	test.Panics(t, func() {
		rt.Attach("service")
	})

	test.Panics(t, func() {
		rt.Set("value")
	})

	test.Panics(t, func() {
		c.SetGlobal("value")
	})

}

func TestRuntimeCreatePanics(t *testing.T) {
	t.Parallel()

	test.Panics(t, func() {
		NewRuntime().Create[int]("created", nil)
	})

	rt := NewRuntime()
	test.NoError(t, rt.Run(t.Context(), RunOptions{NoSignals: true}))

	test.Panics(t, func() {
		rt.Create("created", func(*Container) (string, error) {
			return "value", nil
		})
	})
}

func TestRuntimeRunTwicePanics(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	test.NoError(t, rt.Run(t.Context(), RunOptions{NoSignals: true}))

	test.Panics(t, func() {
		rt.Run(t.Context(), RunOptions{NoSignals: true})
	})
}

func TestRuntimeRunPanics(t *testing.T) {
	t.Parallel()

	test.Panics(t, func() {
		var ctx context.Context
		NewRuntime().Run(ctx)
	})

	test.Panics(t, func() {
		NewRuntime().Run(t.Context(), RunOptions{}, RunOptions{})
	})

}

func TestRuntimeSignalContext(t *testing.T) {
	ctx := t.Context()
	rt := NewRuntime()

	noSignalCtx, stop := rt.signalContext(ctx, RunOptions{NoSignals: true})
	defer stop()
	test.True(t, noSignalCtx == ctx)

	signalCtx, stop := rt.signalContext(ctx, RunOptions{
		Signals: []os.Signal{os.Interrupt},
	})
	test.False(t, signalCtx.Err() != nil)

	stop()
	test.True(t, signalCtx.Err() != nil)

	defaultSignalCtx, defaultStop := rt.signalContext(ctx, RunOptions{})
	test.False(t, defaultSignalCtx.Err() != nil)

	defaultStop()
	test.True(t, defaultSignalCtx.Err() != nil)
}

func TestRuntimeStartStopsWhenContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	rt := NewRuntime()
	rt.ctx = ctx
	rt.startupDone = make(chan struct{})
	c := rt.Attach("service")

	cancel()
	rt.start([]*Container{c})

	waitForSignal(t, rt.startupDone)
	test.Equal(t, containerSetup, c.state)
}

func TestRuntimeStartStopsAfterInitializationCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	rt := NewRuntime()
	rt.ctx = ctx
	rt.startupDone = make(chan struct{})
	c := rt.Attach("service")
	c.OnInit("init", func(*Container) error {
		cancel()
		return nil
	})

	rt.start([]*Container{c})

	waitForSignal(t, rt.startupDone)
	test.Equal(t, runtimeSetup, rt.state)
	test.Equal(t, containerInitializing, c.state)
}

func TestRuntimeStartStopsBetweenContainers(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	rt := NewRuntime()
	rt.ctx = ctx
	rt.startupDone = make(chan struct{})
	first := rt.Attach("first")
	second := rt.Attach("second")

	var secondStarted bool
	first.OnStart("start", func(*Container) error {
		cancel()
		return nil
	})
	second.OnStart("start", func(*Container) error {
		secondStarted = true
		return nil
	})

	rt.start([]*Container{first, second})

	waitForSignal(t, rt.startupDone)
	test.Equal(t, runtimeRunning, rt.state)
	test.Equal(t, containerStarting, first.state)
	test.Equal(t, containerInitialized, second.state)
	test.False(t, secondStarted)
}

func TestContainerRunPanics(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	c := rt.Attach("service")

	test.Panics(t, func() {
		c.Run("", waitForCancel)
	})

	test.Panics(t, func() {
		c.Run("task", nil)
	})

	test.NoError(t, rt.Run(t.Context(), RunOptions{NoSignals: true}))

	test.Panics(t, func() {
		c.Run("task", waitForCancel)
	})

	c.state = containerInitialized
	test.Panics(t, func() {
		c.Run("task", waitForCancel)
	})
}

func TestContainerCtx(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	c := rt.Attach("service")
	c.OnInit("init", func(container *Container) error {
		test.True(t, c == container)
		test.True(t, c.Ctx() == container.Ctx())
		return nil
	})
	c.OnStart("start", func(container *Container) error {
		test.True(t, c == container)
		test.True(t, c.Ctx() == container.Ctx())
		return nil
	})

	test.NoError(t, rt.Run(t.Context(), RunOptions{NoSignals: true}))
}

func TestContainerStartTwicePanics(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	rt.ctx = t.Context()
	c := rt.Attach("service")

	test.NoError(t, c.init())
	test.NoError(t, c.start())

	test.Panics(t, func() {
		c.start()
	})
}

func TestContainerInitTwicePanics(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	rt.ctx = t.Context()
	c := rt.Attach("service")

	test.NoError(t, c.init())

	test.Panics(t, func() {
		c.init()
	})
}

func TestContainerInitStopsWhenContextCanceled(t *testing.T) {
	t.Parallel()

	t.Run("before hook", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		rt := NewRuntime()
		rt.ctx = ctx
		c := rt.Attach("service")

		var initialized bool
		c.OnInit("init", func(*Container) error {
			initialized = true
			return nil
		})

		cancel()

		test.NoError(t, c.init())
		test.False(t, initialized)
		test.Equal(t, containerInitializing, c.state)
	})

	t.Run("after hook", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		rt := NewRuntime()
		rt.ctx = ctx
		c := rt.Attach("service")

		c.OnInit("init", func(*Container) error {
			cancel()
			return nil
		})

		test.NoError(t, c.init())
		test.Equal(t, containerInitializing, c.state)
	})
}

func TestContainerStartStopsWhenContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	rt := NewRuntime()
	rt.ctx = ctx
	c := rt.Attach("service")

	var firstStarted bool
	var secondStarted bool
	c.OnStart("start", func(*Container) error {
		firstStarted = true
		cancel()
		return nil
	})
	c.OnStart("start", func(*Container) error {
		secondStarted = true
		return nil
	})

	test.NoError(t, c.init())
	test.NoError(t, c.start())
	test.True(t, firstStarted)
	test.False(t, secondStarted)
}

func TestContainerHooks(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var events []string
	add := func(event string) {
		events = append(events, event)
	}

	started := make(chan struct{})
	c := rt.Attach("service")
	c.OnInit("init", func(*Container) error {
		add("init")
		c.Set("value")
		return nil
	})
	c.OnInit("init", func(c *Container) error {
		add("init:hooks")
		c.OnStart("start", func(*Container) error {
			add("start:three")
			return nil
		})
		c.OnStopping("stop", func(context.Context, *Container) error {
			add("stop:three")
			return nil
		})
		return nil
	})
	c.OnStart("start", func(*Container) error {
		add("start:one")
		return nil
	})
	c.OnStart("start", func(*Container) error {
		add("start:two")
		close(started)
		return nil
	})
	c.OnStopping("stop", func(context.Context, *Container) error {
		add("stop:one")
		return nil
	})
	c.OnStopping("stop", func(context.Context, *Container) error {
		add("stop:two")
		return nil
	})
	c.Run("wait", waitForCancel)

	errs := make(chan error, 1)
	go func() {
		errs <- rt.Run(ctx, RunOptions{NoSignals: true})
	}()

	waitForSignal(t, started)
	cancel()

	test.NoError(t, waitForRuntime(t, errs))
	test.Equal(t, "value", c.Get[string]())
	test.Equal(t, []string{
		"init",
		"init:hooks",
		"start:one",
		"start:two",
		"start:three",
		"stop:three",
		"stop:two",
		"stop:one",
	}, events)
}

func TestContainerStop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	rt := NewRuntime()
	var events []string
	add := func(event string) {
		events = append(events, event)
	}

	taskStarted := make(chan struct{})
	taskStopped := make(chan struct{})
	c := rt.Attach("service")
	c.Run("wait", func(ctx context.Context) error {
		close(taskStarted)
		<-ctx.Done()
		close(taskStopped)
		return ctx.Err()
	})
	c.OnStopping("one", func(context.Context, *Container) error {
		add("stopping:one")
		return nil
	})
	c.OnStopping("two", func(context.Context, *Container) error {
		add("stopping:two")
		return nil
	})
	c.OnStop("one", func(context.Context, *Container) error {
		select {
		case <-taskStopped:
			add("stop:one")
			return nil
		default:
			return errors.New("stop ran before task stopped")
		}
	})
	c.OnStop("two", func(context.Context, *Container) error {
		select {
		case <-taskStopped:
			add("stop:two")
			return nil
		default:
			return errors.New("stop ran before task stopped")
		}
	})

	errs := make(chan error, 1)
	go func() {
		errs <- rt.Run(ctx, RunOptions{NoSignals: true})
	}()

	waitForSignal(t, taskStarted)
	cancel()

	test.NoError(t, waitForRuntime(t, errs))
	test.Equal(t, []string{
		"stopping:two",
		"stopping:one",
		"stop:two",
		"stop:one",
	}, events)
}

func TestContainerRegistrationPanics(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	c := rt.Attach("service")

	test.Panics(t, func() {
		c.OnInit("", func(*Container) error { return nil })
	})

	test.Panics(t, func() {
		c.OnInit("init", nil)
	})

	test.Panics(t, func() {
		c.OnStart("", func(*Container) error { return nil })
	})

	test.Panics(t, func() {
		c.OnStart("start", nil)
	})

	test.Panics(t, func() {
		c.OnStopping("", func(context.Context, *Container) error { return nil })
	})

	test.Panics(t, func() {
		c.OnStopping("stop", nil)
	})

	test.Panics(t, func() {
		c.OnStop("", func(context.Context, *Container) error { return nil })
	})

	test.Panics(t, func() {
		c.OnStop("stop", nil)
	})

	c.Set(1)
	test.Panics(t, func() {
		c.Set(2)
	})

	test.NoError(t, rt.Run(t.Context(), RunOptions{NoSignals: true}))

	test.Panics(t, func() {
		c.OnInit("init", func(*Container) error { return nil })
	})

	test.Panics(t, func() {
		c.OnStart("start", func(*Container) error { return nil })
	})

	test.Panics(t, func() {
		c.OnStopping("stop", func(context.Context, *Container) error { return nil })
	})

	test.Panics(t, func() {
		c.OnStop("stop", func(context.Context, *Container) error { return nil })
	})

	test.Panics(t, func() {
		c.Set("value")
	})
}

func TestContainerInitializationRunStartsAfterHooks(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	c := rt.Attach("service")

	var started bool
	var taskRan bool
	c.OnInit("init", func(c *Container) error {
		c.Run("task", func(context.Context) error {
			taskRan = true
			if !started {
				return errors.New("task started before hooks")
			}
			return nil
		})
		return nil
	})
	c.OnStart("start", func(*Container) error {
		started = true
		return nil
	})

	test.NoError(t, rt.Run(t.Context(), RunOptions{NoSignals: true}))
	test.True(t, taskRan)
}

func TestContainerConstructionRunStartsAfterHooks(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	c := rt.Attach("service")

	var started bool
	var taskRan bool
	c.Run("task", func(context.Context) error {
		taskRan = true
		if !started {
			return errors.New("task started before hooks")
		}
		return nil
	})
	c.OnStart("start", func(*Container) error {
		started = true
		return nil
	})

	test.NoError(t, rt.Run(t.Context(), RunOptions{NoSignals: true}))
	test.True(t, taskRan)
}

func TestContainerInitializationRunDoesNotStartAfterStartupError(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	errStart := errors.New("startup failed")
	taskStarted := make(chan struct{})
	c := rt.Attach("service")
	c.OnInit("init", func(c *Container) error {
		c.Run("task", func(context.Context) error {
			close(taskStarted)
			return nil
		})
		return nil
	})
	c.OnStart("start", func(*Container) error {
		return errStart
	})

	test.ErrorIs(t, rt.Run(t.Context(), RunOptions{NoSignals: true}), errStart)

	select {
	case <-taskStarted:
		t.Fatal("initialization task started after startup error")
	default:
	}
}

func TestContainerConstructionRunDoesNotStartAfterStartupError(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	errStart := errors.New("startup failed")
	taskStarted := make(chan struct{})
	c := rt.Attach("service")
	c.Run("task", func(context.Context) error {
		close(taskStarted)
		return nil
	})
	c.OnStart("start", func(*Container) error {
		return errStart
	})

	test.ErrorIs(t, rt.Run(t.Context(), RunOptions{NoSignals: true}), errStart)

	select {
	case <-taskStarted:
		t.Fatal("construction task started after startup error")
	default:
	}
}

func TestStore(t *testing.T) {
	t.Parallel()

	s := newStore()
	valueType := reflect.TypeFor[int]()

	_, ok := s.get(valueType)
	test.False(t, ok)
	test.True(t, s.set(valueType, 42))
	test.False(t, s.set(valueType, 43))

	value, ok := s.get(valueType)
	test.True(t, ok)
	test.Equal(t, 42, getValue[int](value))

	test.Nil(t, getValue[any](nil))

	var pointer *int
	test.Nil(t, getValue[*int](pointer))

	test.Panics(t, func() {
		getValue[int]("not an int")
	})
}

func TestStoreZeroValue(t *testing.T) {
	t.Parallel()

	s := &store{}
	valueType := reflect.TypeFor[int]()

	test.True(t, s.set(valueType, 42))
	value, ok := s.get(valueType)
	test.True(t, ok)
	test.Equal(t, 42, getValue[int](value))
}

func TestRuntimeLogger(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	rt := NewRuntime()
	rt.Attach("service").
		OnInit("build", func(*Container) error { return nil }).
		OnStart("ready", func(*Container) error { return nil }).
		OnStopping("close", func(context.Context, *Container) error { return nil }).
		OnStop("release", func(context.Context, *Container) error { return nil }).
		Run("task", func(context.Context) error { return nil })

	test.NoError(t, rt.Run(t.Context(), RunOptions{
		Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
		NoSignals: true,
	}))

	test.Contains(t, logs.String(), `msg="container starting" container=service`)
	test.Contains(
		t,
		logs.String(),
		`msg="container step starting" container=service phase=init step=build`,
	)
	test.Contains(
		t,
		logs.String(),
		`msg="container step completed" container=service phase=init step=build`,
	)
	test.Contains(
		t,
		logs.String(),
		`msg="container step starting" container=service phase=start step=ready`,
	)
	test.Contains(
		t,
		logs.String(),
		`msg="container step completed" container=service phase=start step=ready`,
	)
	test.Contains(t, logs.String(), `msg="container task starting" container=service task=task`)
	test.Contains(t, logs.String(), `msg="container task stopped" container=service task=task`)
	test.Contains(
		t,
		logs.String(),
		`msg="container step starting" container=service phase=stopping step=close`,
	)
	test.Contains(
		t,
		logs.String(),
		`msg="container step completed" container=service phase=stopping step=close`,
	)
	test.Contains(
		t,
		logs.String(),
		`msg="container step starting" container=service phase=stop step=release`,
	)
	test.Contains(
		t,
		logs.String(),
		`msg="container step completed" container=service phase=stop step=release`,
	)
	test.Contains(t, logs.String(), `msg="container stopped" container=service`)
}

func TestRuntimeLoggerStepError(t *testing.T) {
	t.Parallel()

	errStep := errors.New("load failed")
	var logs bytes.Buffer
	rt := NewRuntime()
	rt.Attach("service").OnStart("load", func(*Container) error {
		return errStep
	})

	err := rt.Run(t.Context(), RunOptions{
		Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
		NoSignals: true,
	})

	test.ErrorIs(t, err, errStep)
	test.ErrorMessage(t, err, "service: load: load failed")
	test.Contains(
		t,
		logs.String(),
		`msg="container step error" container=service phase=start step=load err="load failed"`,
	)
}

func TestRuntimeLoggerTaskError(t *testing.T) {
	t.Parallel()

	errTask := errors.New("task failed")
	started := make(chan struct{})
	var logs bytes.Buffer
	rt := NewRuntime()
	rt.Attach("service").
		Run("waiting", func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}).
		Run("failing", func(context.Context) error {
			<-started
			return errTask
		})

	err := rt.Run(t.Context(), RunOptions{
		Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
		NoSignals: true,
	})

	test.ErrorIs(t, err, errTask)
	test.ErrorMessage(t, err, "service: failing: task failed")
	test.Contains(
		t,
		logs.String(),
		`msg="container task error" container=service task=failing err="task failed"`,
	)
	test.Contains(t, logs.String(), `msg="container task stopped" container=service task=waiting`)
}

func TestRuntimeShutdownWatchdog(t *testing.T) {
	t.Run("stops when no work remains", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const timeout = 25 * time.Millisecond

			rt := NewRuntime()
			rt.ctx = t.Context()
			rt.shutdownTimeout = timeout

			done := make(chan struct{})
			stopped := make(chan struct{})
			go func() {
				rt.watchShutdown(done)
				close(stopped)
			}()

			synctest.Wait()
			synctest.Sleep(timeout)
			<-stopped
		})
	})

	t.Run("does not log after work exits", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const timeout = 250 * time.Millisecond

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			started := make(chan struct{})
			var logs lockedBuffer
			rt := NewRuntime()
			rt.Attach("service").Run("task", func(ctx context.Context) error {
				close(started)
				<-ctx.Done()
				return ctx.Err()
			})

			errs := make(chan error, 1)
			go func() {
				errs <- rt.Run(ctx, RunOptions{
					Logger:          slog.New(slog.NewTextHandler(&logs, nil)),
					NoSignals:       true,
					ShutdownTimeout: timeout,
				})
			}()

			synctest.Wait()
			<-started
			cancel()
			synctest.Wait()
			test.NoError(t, <-errs)

			synctest.Sleep(2 * timeout)
			test.False(t, strings.Contains(logs.String(), "runtime shutdown timeout exceeded"))
		})
	})

	t.Run("logs outstanding work and keeps waiting", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const timeout = 25 * time.Millisecond

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			started := make(chan struct{})
			release := make(chan struct{})
			var logs lockedBuffer
			rt := NewRuntime()
			rt.Attach("api").Run("server", func(context.Context) error {
				close(started)
				<-release
				return nil
			})

			errs := make(chan error, 1)
			go func() {
				errs <- rt.Run(ctx, RunOptions{
					Logger:          slog.New(slog.NewTextHandler(&logs, nil)),
					NoSignals:       true,
					ShutdownTimeout: timeout,
				})
			}()

			synctest.Wait()
			<-started
			cancel()
			synctest.Wait()
			synctest.Sleep(2 * timeout)

			close(release)
			synctest.Wait()
			test.NoError(t, <-errs)
			test.Equal(t, 2, logCount(logs.String(), `container=api task=server`))
			test.Contains(
				t,
				logs.String(),
				`msg="runtime shutdown timeout exceeded while waiting for managed work"`,
			)
			test.Contains(t, logs.String(), "elapsed=25ms")
			test.Contains(t, logs.String(), "elapsed=50ms")
		})
	})

	t.Run("logs each outstanding task", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const timeout = 25 * time.Millisecond

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			firstStarted := make(chan struct{})
			secondStarted := make(chan struct{})
			firstRelease := make(chan struct{})
			secondRelease := make(chan struct{})
			var logs lockedBuffer
			rt := NewRuntime()
			first := rt.Attach("first")
			first.Run("worker", func(context.Context) error {
				close(firstStarted)
				<-firstRelease
				return nil
			})
			rt.Attach("second").Run("server", func(context.Context) error {
				close(secondStarted)
				<-secondRelease
				return nil
			})

			errs := make(chan error, 1)
			go func() {
				errs <- rt.Run(ctx, RunOptions{
					Logger:          slog.New(slog.NewTextHandler(&logs, nil)),
					NoSignals:       true,
					ShutdownTimeout: timeout,
				})
			}()

			synctest.Wait()
			<-firstStarted
			<-secondStarted
			cancel()
			synctest.Wait()
			synctest.Sleep(timeout)

			close(firstRelease)
			synctest.Wait()
			test.Len(t, 0, first.runningTasks())
			synctest.Sleep(timeout)
			test.Equal(t, 1, logCount(logs.String(), `container=first task=worker`))
			test.Equal(t, 2, logCount(logs.String(), `container=second task=server`))

			close(secondRelease)
			synctest.Wait()
			test.NoError(t, <-errs)
		})
	})
}

func TestRuntimeTaskContextCancellation(t *testing.T) {
	t.Parallel()

	errTask := errors.New("task failed")
	started := make(chan struct{})
	cause := make(chan error, 1)
	rt := NewRuntime()
	rt.Attach("service").
		Run("waiting", func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			cause <- context.Cause(ctx)
			return ctx.Err()
		}).
		Run("failing", func(context.Context) error {
			<-started
			return errTask
		})

	err := rt.Run(t.Context(), RunOptions{NoSignals: true})

	test.ErrorIs(t, err, errTask)
	test.Equal(t, context.Canceled, <-cause)
}

func TestRuntimeTaskContextParentCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)

	started := make(chan struct{})
	cause := make(chan error, 1)
	rt := NewRuntime()
	rt.Attach("service").Run("waiting", func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		cause <- context.Cause(ctx)
		return ctx.Err()
	})

	errs := make(chan error, 1)
	go func() {
		errs <- rt.Run(ctx, RunOptions{NoSignals: true})
	}()

	waitForSignal(t, started)
	cancel(errors.New("parent failed"))

	test.NoError(t, waitForRuntime(t, errs))
	test.Equal(t, context.Canceled, <-cause)
}

func TestNewRuntimeTaskContext(t *testing.T) {
	t.Parallel()

	type key struct{}
	parent, cancelParent := context.WithCancelCause(
		context.WithValue(t.Context(), key{}, "value"),
	)
	defer cancelParent(nil)

	ctx, cancel := newRuntimeTaskContext(parent)
	defer cancel()

	test.Equal(t, "value", ctx.Value(key{}))
	cancelParent(errors.New("parent failed"))
	waitForSignal(t, ctx.Done())
	test.Equal(t, context.Canceled, context.Cause(ctx))
}

func TestNewRuntimeTaskContextDeadline(t *testing.T) {
	t.Parallel()

	parent, cancelParent := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancelParent()

	ctx, cancel := newRuntimeTaskContext(parent)
	defer cancel()

	parentDeadline, ok := parent.Deadline()
	test.True(t, ok)
	deadline, ok := ctx.Deadline()
	test.True(t, ok)
	test.Equal(t, parentDeadline, deadline)

	waitForSignal(t, ctx.Done())
	test.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}

func TestRuntimeStopError(t *testing.T) {
	t.Parallel()

	errStop := errors.New("release failed")
	rt := NewRuntime()
	rt.Attach("service").OnStop("release", func(context.Context, *Container) error {
		return errStop
	})

	err := rt.Run(t.Context(), RunOptions{NoSignals: true})

	test.ErrorIs(t, err, errStop)
	test.ErrorMessage(t, err, "service: release: release failed")
}

func TestRuntimeLifecycle(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var events []string
	var mu sync.Mutex
	add := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}

	first := rt.Attach("first").OnStart("start", func(*Container) error {
		add("first:start")
		return nil
	}).OnStopping("stop", func(context.Context, *Container) error {
		add("first:stop")
		return nil
	})
	first.Run("wait", waitForCancel)

	started := make(chan struct{})
	second := rt.Attach("second").OnStart("start", func(*Container) error {
		add("second:start")
		close(started)
		return nil
	}).OnStopping("stop", func(context.Context, *Container) error {
		add("second:stop")
		return nil
	})
	second.Run("wait", waitForCancel)

	errs := make(chan error, 1)
	go func() {
		errs <- rt.Run(ctx, RunOptions{NoSignals: true})
	}()

	waitForSignal(t, started)
	cancel()

	test.NoError(t, waitForRuntime(t, errs))
	test.Equal(t, []string{
		"first:start",
		"second:start",
		"second:stop",
		"first:stop",
	}, events)
}

func TestRuntimeInitialization(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	var events []string
	add := func(event string) {
		events = append(events, event)
	}

	rt.Attach("first").OnInit("init", func(c *Container) error {
		add("first:init")
		c.SetGlobal("runtime value")
		c.Set(42)
		return nil
	}).OnStart("start", func(c *Container) error {
		add("first:start")
		test.Equal(t, 42, c.Get[int]())
		return nil
	})

	rt.Attach("second").OnInit("init", func(c *Container) error {
		add("second:init")
		test.Equal(t, "runtime value", c.Global[string]())
		return nil
	}).OnStart("start", func(*Container) error {
		add("second:start")
		return nil
	})

	test.NoError(t, rt.Run(t.Context(), RunOptions{NoSignals: true}))
	test.Equal(t, []string{
		"first:init",
		"second:init",
		"first:start",
		"second:start",
	}, events)
}

func TestRuntimeInitializationError(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	errInit := errors.New("initialization failed")

	var firstStarted bool
	var firstStopped bool
	rt.Attach("first").OnInit("init", func(*Container) error {
		return nil
	}).OnStart("start", func(*Container) error {
		firstStarted = true
		return nil
	}).OnStopping("stop", func(context.Context, *Container) error {
		firstStopped = true
		return nil
	})

	var nextInitialized bool
	rt.Attach("failing").OnInit("init", func(*Container) error {
		return errInit
	})
	rt.Attach("next").OnInit("init", func(*Container) error {
		nextInitialized = true
		return nil
	})

	err := rt.Run(t.Context(), RunOptions{NoSignals: true})

	test.ErrorIs(t, err, errInit)
	test.False(t, firstStarted)
	test.True(t, firstStopped)
	test.False(t, nextInitialized)
}

func TestRuntimeStartupError(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	errStart := errors.New("startup failed")

	var stopped bool
	rt.Attach("failing").OnStart("start", func(*Container) error {
		return errStart
	}).OnStopping("stop", func(context.Context, *Container) error {
		stopped = true
		return nil
	})

	var nextStarted bool
	rt.Attach("next").OnStart("start", func(*Container) error {
		nextStarted = true
		return nil
	})

	err := rt.Run(t.Context(), RunOptions{NoSignals: true})

	test.ErrorIs(t, err, errStart)
	test.True(t, stopped)
	test.False(t, nextStarted)
}

func TestRuntimeTaskErrorTakesPrecedenceOverStopError(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	errTask := errors.New("task failed")
	errStop := errors.New("stop failed")

	var stopped bool
	c := rt.Attach("service").OnStopping("stop", func(context.Context, *Container) error {
		stopped = true
		return errStop
	})
	c.OnStart("start", func(*Container) error {
		c.Run("failing", func(context.Context) error {
			return errTask
		})
		return nil
	})

	err := rt.Run(t.Context(), RunOptions{NoSignals: true})

	test.ErrorIs(t, err, errTask)
	test.True(t, stopped)
}

func TestRuntimeStoppingError(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errStop := errors.New("stop failed")
	started := make(chan struct{})

	c := rt.Attach("service").OnStart("start", func(*Container) error {
		close(started)
		return nil
	}).OnStopping("stop", func(context.Context, *Container) error {
		return errStop
	})
	c.Run("wait", waitForCancel)

	errs := make(chan error, 1)
	go func() {
		errs <- rt.Run(ctx, RunOptions{NoSignals: true})
	}()

	waitForSignal(t, started)
	cancel()

	test.ErrorIs(t, waitForRuntime(t, errs), errStop)
}

func TestRuntimeShutdownWaitsForStartup(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	started := make(chan struct{})
	startupDone := make(chan struct{})
	errStopBeforeStartupDone := errors.New("stop before startup completed")

	rt.Attach("service").OnStart("start", func(c *Container) error {
		close(started)
		<-c.Ctx().Done()
		close(startupDone)
		return nil
	}).OnStopping("stop", func(context.Context, *Container) error {
		select {
		case <-startupDone:
			return nil
		default:
			return errStopBeforeStartupDone
		}
	})

	errs := make(chan error, 1)
	go func() {
		errs <- rt.Run(ctx, RunOptions{NoSignals: true})
	}()

	waitForSignal(t, started)
	cancel()

	test.NoError(t, waitForRuntime(t, errs))
}

func waitForCancel(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func waitForRuntime(t *testing.T, errs <-chan error) error {
	t.Helper()

	select {
	case err := <-errs:
		return err
	case <-time.After(time.Second):
		t.Fatal("Runtime.Run did not return")
		return nil
	}
}

func waitForSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("signal was not received")
	}
}

func logCount(logs string, value string) int {
	count := 0
	for line := range strings.SplitSeq(logs, "\n") {
		if strings.Contains(
			line,
			"runtime shutdown timeout exceeded while waiting for managed work",
		) &&
			strings.Contains(line, value) {
			count++
		}
	}

	return count
}
