// Web demonstrates a small REST API built with gx/web.
//
// Run with:
//
//	go run ./examples/web
//
// Then try:
//
//	curl http://localhost:8080/health
//	curl -X POST http://localhost:8080/tasks -H 'Content-Type: application/json' -d '{"title":"write docs"}'
//	curl 'http://localhost:8080/tasks?pretty'
//
// Press Ctrl-C (or send SIGTERM) to shut the server down gracefully.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/shayanderson/gx/web"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	server := web.NewServer(web.Options{
		// Addr is the HTTP address to listen on.
		Addr: ":8080",

		// Logger enables request, error, startup, and shutdown logs.
		Logger:    logger,
		LogPrefix: "tasks-api",

		// MaxReadSize limits JSON request bodies accepted by Context.Bind.
		MaxReadSize: 1 << 20, // 1 MB

		// These limits are passed to the underlying net/http server.
		IdleTimeout:       time.Minute,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,

		// ShutdownTimeout bounds the graceful shutdown of in-flight requests.
		ShutdownTimeout: 6 * time.Second,

		// ErrorHandler turns handler errors into a consistent JSON shape. If it
		// is omitted, gx/web writes {"error":"..."} automatically.
		ErrorHandler: writeError,
	})

	// Global middleware runs around every route. It must be registered before
	// requests are served. This one assigns a request ID, stores it in Context,
	// and returns it to the client.
	server.Use(requestIDMiddleware())

	api := newTaskAPI()
	api.mount(server)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Start() }()

	select {
	case err := <-serverErr:
		if err != nil {
			logger.Error("server failed", slog.String("err", err.Error()))
		}
	case <-ctx.Done():
		logger.Info("shutdown requested")
		if err := server.Stop(); err != nil {
			logger.Error("graceful shutdown failed", slog.String("err", err.Error()))
		}
		if err := <-serverErr; err != nil {
			logger.Error("server stopped with an error", slog.String("err", err.Error()))
		}
	}
}

// Task is the JSON resource returned by the API.
type Task struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"createdAt"`
}

type createTaskRequest struct {
	Title string `json:"title"`
}

type updateTaskRequest struct {
	Title *string `json:"title"`
	Done  *bool   `json:"done"`
}

// taskAPI uses an in-memory store so the example is self-contained.
type taskAPI struct {
	nextID int
	mu     sync.RWMutex
	tasks  map[int]Task
}

func newTaskAPI() *taskAPI {
	return &taskAPI{nextID: 1, tasks: make(map[int]Task)}
}

func (a *taskAPI) mount(server *web.Server) {
	server.GET("/health", func(c *web.Context) error {
		return c.JSON(web.Map{"status": "ok"})
	})
	server.GET("/tasks", a.list)
	server.POST("/tasks", a.create)
	server.GET("/tasks/{id}", a.get)
	server.PATCH("/tasks/{id}", a.update)
	server.DELETE("/tasks/{id}", a.delete)
}

func (a *taskAPI) list(c *web.Context) error {
	a.mu.RLock()
	tasks := make([]Task, 0, len(a.tasks))
	for _, task := range a.tasks {
		tasks = append(tasks, task)
	}
	a.mu.RUnlock()
	slices.SortFunc(tasks, func(a, b Task) int { return a.ID - b.ID })

	return c.JSON(web.Map{"tasks": tasks})
}

func (a *taskAPI) create(c *web.Context) error {
	var input createTaskRequest
	if err := c.Bind(&input); err != nil {
		return err
	}

	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return web.Error(http.StatusBadRequest, "title is required")
	}

	a.mu.Lock()
	task := Task{
		ID:        a.nextID,
		Title:     input.Title,
		CreatedAt: time.Now().UTC(),
	}
	a.tasks[task.ID] = task
	a.nextID++
	a.mu.Unlock()

	c.Writer().Header().Set("Location", fmt.Sprintf("/tasks/%d", task.ID))
	return c.JSON(task, http.StatusCreated)
}

func (a *taskAPI) get(c *web.Context) error {
	task, err := a.find(c.Request.PathValue("id"))
	if err != nil {
		return err
	}
	return c.JSON(task)
}

func (a *taskAPI) update(c *web.Context) error {
	id, err := taskID(c.Request.PathValue("id"))
	if err != nil {
		return err
	}

	var input updateTaskRequest
	if err := c.Bind(&input); err != nil {
		return err
	}
	if input.Title == nil && input.Done == nil {
		return web.Error(http.StatusBadRequest, "supply title or done")
	}

	a.mu.Lock()
	task, ok := a.tasks[id]
	if !ok {
		a.mu.Unlock()
		return web.Error(http.StatusNotFound, "task not found")
	}
	if input.Title != nil {
		task.Title = strings.TrimSpace(*input.Title)
		if task.Title == "" {
			a.mu.Unlock()
			return web.Error(http.StatusBadRequest, "title cannot be empty")
		}
	}
	if input.Done != nil {
		task.Done = *input.Done
	}
	a.tasks[id] = task
	a.mu.Unlock()

	return c.JSON(task)
}

func (a *taskAPI) delete(c *web.Context) error {
	id, err := taskID(c.Request.PathValue("id"))
	if err != nil {
		return err
	}

	a.mu.Lock()
	_, ok := a.tasks[id]
	delete(a.tasks, id)
	a.mu.Unlock()
	if !ok {
		return web.Error(http.StatusNotFound, "task not found")
	}

	c.Status(http.StatusNoContent)
	return nil
}

func (a *taskAPI) find(rawID string) (Task, error) {
	id, err := taskID(rawID)
	if err != nil {
		return Task{}, err
	}

	a.mu.RLock()
	task, ok := a.tasks[id]
	a.mu.RUnlock()
	if !ok {
		return Task{}, web.Error(http.StatusNotFound, "task not found")
	}
	return task, nil
}

func taskID(raw string) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id < 1 {
		return 0, web.Error(http.StatusBadRequest, "invalid task id")
	}
	return id, nil
}

type requestIDKey struct{}

func requestIDMiddleware() web.Middleware {
	var next atomic.Uint64
	return func(nextHandler web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			requestID := fmt.Sprintf("req-%d", next.Add(1))
			c.Set(requestIDKey{}, requestID)
			c.Writer().Header().Set("X-Request-ID", requestID)
			return nextHandler(c)
		}
	}
}

func writeError(c *web.Context, err web.StatusError) {
	// The handler is only called before a response has started. Preserve the
	// status selected by web.Error, Errorf, ErrorStatus, or ErrorWrap.
	_ = c.JSON(web.Map{
		"error":     err.Error(),
		"requestId": c.Get(requestIDKey{}),
	}, err.Status())
}
