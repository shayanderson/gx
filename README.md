# gx

`gx` is a collection of small, focused packages and types that complement the Go standard library. It provides functionality that feels like a natural extension of the standard library while remaining idiomatic, lightweight, and dependency-free.

## Installation

```bash
go get github.com/shayanderson/gx
```

## Documentation

See the Go package documentation for complete APIs and examples. See tests for additional usage examples.

## Packages

- [`gx`](#gx)
- [`assert`](#assert)
- [`env`](#env)
- [`test`](#test)
- [`web`](#web)

### gx

- [`Accumulator`](#accumulator)
- [`Buffer[T]`](#buffert)
- [`Bus`](#bus)
- [`Debouncer`](#debouncer)
- [`Dispatcher`](#dispatcher)
- [`Map[K, V]`](#mapk-v)
- [`Queue[T]`](#queuet)
- [`Retry`](#retry)
- [`Runner`](#runner)
- [`Runtime`](#runtime)
- [`Semaphore`](#semaphore)
- [`Set[T]`](#sett)
- [`Stack[T]`](#stackt)
- [`Throttler`](#throttler)
- [`WaitQueue[T]`](#waitqueuet)

#### `Accumulator`

Accumulates values and flushes every interval or at a threshold. Zero totals are not flushed.

```go
a, err := gx.NewAccumulator(ctx, gx.AccumulatorOptions{
    Delay: 1*time.Second,
    Max:   100,
    Flush: func(total int) {
        fmt.Println(total)
    },
})
a.Add(25)
a.Add(75) // flushes
```

See the [Accumulator example](examples/accumulator/main.go) for a complete runnable demonstration.

#### `Buffer[T]`

Concurrency-safe, dynamically growing FIFO buffer.

```go
b := gx.NewBuffer[string](16)
b.Push("one")
b.Push("two")

go func() {
    for {
        // Block until values are available or the buffer is closed.
        value, ok := b.Next()
        if !ok {
            return
        }
        fmt.Println(value)
    }
}()

b.Push("three")
b.Close()
```

Use `TryNext()` to read without blocking:

```go
if value, ok := b.TryNext(); ok {
    fmt.Println(value)
}
```

See the [Buffer example](examples/buffer/main.go) for a complete runnable demonstration.

#### `Bus`

Publishes typed values asynchronously to registered subscribers. Use `Bus` when subscribers should run asynchronously from the publisher. `Publish` panics if no subscribers are registered for the published type.

```go
type UserLoggedIn struct {
    UserID string
}

type UserLoggedOut struct {
    UserID string
}

b := gx.NewBus(4)
b.Subscribe(func(ctx context.Context, e UserLoggedIn) {
    fmt.Println("user logged in:", e.UserID)
})
b.Subscribe(func(ctx context.Context, e UserLoggedOut) {
    fmt.Println("user logged out:", e.UserID)
})

b.Publish(ctx, UserLoggedIn{UserID: "u123"})
b.Publish(ctx, UserLoggedOut{UserID: "u123"})
```

See the [Bus example](examples/bus/main.go) for a complete runnable demonstration.

#### `Debouncer`

Delays execution until no new calls occur within the configured interval.

```go
d := gx.NewDebouncer(250 * time.Millisecond)
d.Do(func() {
    save()
})
```

See the [Debouncer example](examples/debouncer/main.go) for a complete runnable demonstration.

#### `Dispatcher`

Dispatches typed values to registered handlers. Use `Dispatcher` when handlers should run synchronously with the caller.

```go
type UserLoggedIn struct {
    UserID string
}

type UserLoggedOut struct {
    UserID string
}

d := gx.NewDispatcher()
d.Register(func(ctx context.Context, e UserLoggedIn) error {
    fmt.Println("user logged in:", e.UserID)
    return nil
})
d.Register(func(ctx context.Context, e UserLoggedOut) error {
    fmt.Println("user logged out:", e.UserID)
    return nil
})

err := d.Dispatch(ctx, UserLoggedIn{UserID: "u123"})
err = d.Dispatch(ctx, UserLoggedOut{UserID: "u123"})
```

See the [Dispatcher example](examples/dispatcher/main.go) for direct synchronous dispatch, or the [Dispatcher-Queue example](examples/dispatcher-queue/main.go) for buffered dispatch.

#### `Map[K, V]`

Generic concurrency-safe map.

```go
m := gx.NewMap[string, int]()
m.Set("count", 1)
count, ok := m.Get("count")
```

#### `Queue[T]`

Processes items using a pool of workers.

```go
worker := func(ctx context.Context, item int) error {
    fmt.Println("processing:", item)
    return nil
}
q := gx.NewQueue(gx.QueueOptions[int]{
    Size: 128,
    Worker: worker,
    Workers: 2,
})

go func() {
    if err := q.Run(ctx); err != nil {
        fmt.Println("queue error:", err)
    }
}()

ok := q.Push(42)
q.Close() // no more items; Run returns after processing 42
```

See the [Queue example](examples/queue/main.go) for a complete runnable demonstration.

`Run` blocks until the context is canceled, a worker returns an error, or all workers stop. After `Close`, workers process remaining buffered items and stop once the queue is empty, so `Run` returns `nil`.

#### `Retry`

Retries a function using configurable limits, delay, and backoff. With no attempts or max duration, it retries until the context is done.

```go
r, err := gx.NewRetry(gx.RetryOptions{
    Delay:    time.Second,
    Backoff:  2, // optional exponential backoff
    MaxDuration: 30 * time.Second,
})
err = r.Do(ctx, func(ctx context.Context) error {
    return callAPI(ctx)
})
```

See the [Retry example](examples/retry/main.go) for a complete runnable demonstration.

#### `Runner`

Runs concurrent tasks with automatic context cancellation on error.

```go
r, ctx := gx.NewRunner(ctx)
r.Run(func() error { return work(ctx) })
r.Run(func() error { return work2(ctx) })
err := r.Wait()
```

See the [Runner example](examples/runner/main.go) for a complete runnable demonstration.

#### `Runtime`

Manages an application's service lifecycle: initialization, startup, task
cancellation, and reverse-order shutdown. A Runtime is single-use.

```go
rt := gx.NewRuntime()
rt.Create("api", newAPI)

err := rt.Run(ctx, gx.RunOptions{
    Logger: slog.Default(),
    ShutdownTimeout: 10 * time.Second,
})

func newAPI(c *gx.Container) (*API, error) {
	api := &API{}
	c.Run("server", api.Run)
	c.OnStop("close server", api.Close)
	return api, nil
}
```

See the [Runtime example](examples/runtime/main.go) for a complete runnable application.

Main container methods:

- `OnInit` builds and wires the container before any container starts.
- `OnStart` synchronously brings the container online before queued work starts.
- `Run` registers managed concurrent work. Queued work starts after all `OnStart` hooks succeed.
- `OnStopping` runs during shutdown and before managed work exits. Use it to signal or stop work that must exit before finalization.
- `OnStop` runs after managed work exits and safely finalizes resources.

#### `Semaphore`

Limits concurrent access to a resource. Acquire blocks until a slot is available or the context is done.

```go
sem := gx.NewSemaphore(4)

if err := sem.Acquire(ctx); err != nil {
    return err
}
defer sem.Release()

work()
```

#### `Set[T]`

Generic concurrency-safe set.

```go
s := gx.NewSet("a", "b")
s.Add("c")
ok := s.Has("b")
```

#### `Stack[T]`

Concurrency-safe, dynamically growing LIFO stack.

```go
s := gx.NewStack[string](16)
s.Push("one")
s.Push("two")

go func() {
    for {
        // Block until values are available or the stack is closed.
        value, ok := s.Pop()
        if !ok {
            return
        }
        fmt.Println(value)
    }
}()

s.Push("three")
s.Close()
```

Use `TryPop()` to read without blocking:

```go
if value, ok := s.TryPop(); ok {
    fmt.Println(value)
}
```

See the [Stack example](examples/stack/main.go) for a complete runnable demonstration.

#### `Throttler`

Limits how often an action may execute.

```go
t := gx.NewThrottler(time.Second)
t.Do(func() {
    refresh()
})
```

See the [Throttler example](examples/throttler/main.go) for a complete runnable demonstration.

Alternatively, use `Allow()` to control execution manually.

#### `WaitQueue[T]`

Processes items using a pool of workers with cancellable producer backpressure.

```go
q := gx.NewWaitQueue(gx.WaitQueueOptions[int]{
    Size: 128,
    Worker: func(ctx context.Context, item int) error {
        fmt.Println("processing:", item)
        return nil
    },
    Workers: 2,
})

go func() {
    if err := q.Run(ctx); err != nil {
        fmt.Println("queue error:", err)
    }
}()

if err := q.PushWait(ctx, 42); err != nil {
    fmt.Println("could not queue item:", err)
}
q.Close() // no more items, Run returns after processing 42
```

`Push` returns `false` when capacity is unavailable. `PushWait` waits for capacity, context cancellation, or `Close`; a nil error means the item was enqueued, not processed. `WaitQueueOptions` intentionally has no `FailOnFull` option.

See the [WaitQueue example](examples/wait-queue/main.go) for a complete runnable demonstration.

### assert

Runtime assertions that panic with a stack trace.

```go
assert.Equal(200, statusCode)
assert.NoError(err)
assert.True(user.Active)

// Optional message.
assert.Equal(200, statusCode, "unexpected status code")
```

### env

Helpers for reading and parsing environment variables.

```go
port := env.Int("PORT", 8080)
debug := env.Bool("DEBUG", false)

apiKey := env.MustString("API_KEY")
timeout := env.MustDuration("HTTP_TIMEOUT")
```

### test

Testing assertions that fail the current test with `t.Fatal`.

```go
func TestThing(t *testing.T) {
    test.Equal(t, "expected", got)
    test.NoError(t, err)
    test.True(t, ok)

    // Optional message.
    test.Equal(t, "expected", got, "unexpected value")
}
```

### web

Lightweight HTTP toolkit built on Go's standard `net/http` package.

```go
s := web.NewServer(web.Options{
    Addr: ":8080",
})

s.GET("/", func(c *web.Context) error {
    return c.JSON(web.Map{
        "status": "ok",
    })
})

s.GET("/users/{id}", func(c *web.Context) error {
    user, err := store.Get(c.Context(), c.Request.PathValue("id"))
    if err != nil {
        return web.Error(http.StatusNotFound, "user not found")
    }

    return c.JSON(user)
})

s.POST("/users", func(c *web.Context) error {
    var input struct {
        Name string `json:"name"`
    }
    if err := c.Bind(&input); err != nil {
        return err
    }

    return c.JSON(input, http.StatusCreated)
})
```

See the [web example](examples/web/main.go) for a complete runnable REST API.

Handler error messages are returned to clients. For an error that should not be public, log the
details in the application and return `web.ErrorStatus()`, which returns a 500 Internal Server
Error response. Pass a status to return its generic message, such as
`web.ErrorStatus(http.StatusServiceUnavailable)`.

Logging is opt-in: set `Logger` to receive request and error logs or leave it nil to disable
logging. `LogPrefix` defaults to `"http"` when a logger is configured.

`MaxReadSize` limits the bytes `Context.Bind` reads from a request body. A value of `0` uses the
5 MB default. Set a negative value to disable the limit, or a positive value to set a custom limit.

`ShutdownTimeout` controls how long `Stop` waits for in-flight requests and defaults to 2 seconds.
