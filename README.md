# gx

`gx` is a collection of small, focused packages and types that complement Go's standard library. It provides functionality that feels like a natural extension of the standard library while remaining idiomatic, lightweight, and dependency-free.

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
- [`Semaphore`](#semaphore)
- [`Set[T]`](#sett)
- [`Stack[T]`](#stackt)
- [`Throttler`](#throttler)

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

#### `Bus`

Publishes typed values asynchronously to registered subscribers. Use `Bus` when subscribers should run asynchronously from the publisher.

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

#### `Debouncer`

Delays execution until no new calls occur within the configured interval.

```go
d := gx.NewDebouncer(250 * time.Millisecond)
d.Do(func() {
    save()
})
```

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

<details>
<summary>Buffered event dispatching with Queue[T]</summary>

```go
type Event interface {
    dispatch(context.Context, *gx.Dispatcher) error
}

type UserLoggedIn struct {
    UserID string
}

func (e UserLoggedIn) dispatch(ctx context.Context, d *gx.Dispatcher) error {
    return d.Dispatch(ctx, e)
}

type UserLoggedOut struct {
    UserID string
}

func (e UserLoggedOut) dispatch(ctx context.Context, d *gx.Dispatcher) error {
    return d.Dispatch(ctx, e)
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

q := gx.NewQueue(gx.QueueOptions[Event]{
    Worker: func(ctx context.Context, e Event) error {
        // Dispatch through the concrete event type so Dispatcher
        // can resolve the correct handler.
        return e.dispatch(ctx, d)
    },
})

go q.Run(ctx)

q.Push(UserLoggedIn{UserID: "u123"})
q.Push(UserLoggedOut{UserID: "u123"})
```

</details>

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
go q.Run(ctx) // start workers
ok := q.Push(42)
```

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

#### `Runner`

Runs concurrent tasks with automatic context cancellation on error.

```go
r, ctx := gx.NewRunner(ctx)
r.Run(func() error { return work(ctx) })
r.Run(func() error { return work2(ctx) })
err := r.Wait()
```

#### `Semaphore`

Limits concurrent access to a resource.

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

#### `Throttler`

Limits how often an action may execute.

```go
t := gx.NewThrottler(time.Second)
t.Do(func() {
    refresh()
})
```

Alternatively, use `Allow()` to control execution manually.

### assert

Runtime assertions that panic with a stack trace.

```go
assert.Equal(200, statusCode)
assert.NoError(err)
assert.True(user.Active)
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
}
```

### web

Lightweight HTTP toolkit built on Go's standard `net/http` package.

```go
s := web.NewServer(web.Options{
    Addr: ":8080",
})

s.Get("/", func(c *web.Context) error {
    return c.JSON(map[string]any{
        "status": "ok",
    })
})

s.Get("/users/{id}", func(c *web.Context) error {
    user, err := store.Get(c.Context(), c.Request.PathValue("id"))
    if err != nil {
        return web.ErrorWrap(http.StatusNotFound, err)
    }

    return c.JSON(user)
})
```
