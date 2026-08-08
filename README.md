# gx

`gx` is a collection of small, focused packages and types that complement Go's standard library. It provides functionality that feels like a natural extension of the standard library while remaining idiomatic and lightweight.

## Installation

```bash
go get github.com/shayanderson/gx
```

## Documentation

See the Go package documentation for complete APIs and examples. See tests for additional usage examples.

## Packages

### gx

#### `Accumulator`

Accumulates values and flushes on threshold or timeout.

```go
a := gx.NewAccumulator(ctx, time.Second, 100, func(total int) {
    fmt.Println(total)
})
a.Add(25)
a.Add(75) // flushes
```

#### `Map[K, V]`

Generic concurrency-safe map.

```go
m := gx.NewMap[string, int]()
m.Set("count", 1)
count, _ := m.Get("count")
```

#### `Queue[T]`

Processes items using a pool of workers.

```go
worker := func(ctx context.Context, item int) error {
    fmt.Println("processing:", item)
    return nil
}
q := gx.NewQueue(gx.JobQueueOptions[int]{Worker: worker})
go q.Run(ctx) // start workers
q.Push(42)
```

#### `Runner`

Runs concurrent tasks with automatic context cancellation on error.

```go
r, ctx := gx.NewRunner(ctx)
r.Run(func() error { return work(ctx) })
r.Run(func() error { return work2(ctx) })
err := r.Wait()
```

#### `Set[T]`

Generic concurrency-safe set.

```go
s := gx.NewSet("a", "b")
s.Add("c")
ok := s.Has("b")
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
