package gx

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Subscriber is a function that handles a published value of type T.
type Subscriber[T any] func(context.Context, T)

// Bus publishes typed values to registered subscribers asynchronously.
type Bus struct {
	m   map[reflect.Type][]any
	mu  sync.RWMutex
	sem chan struct{}
}

// NewBus creates a new Bus with the specified maximum number of concurrent
// publishes. If maxConcurrency is zero or negative, it defaults to 1.
func NewBus(maxConcurrency int) *Bus {
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}

	return &Bus{
		m:   make(map[reflect.Type][]any),
		sem: make(chan struct{}, maxConcurrency),
	}
}

// Publish publishes value asynchronously to all subscribers registered for
// type T. It blocks until a concurrency slot is available or ctx is done.
func (b *Bus) Publish[T any](ctx context.Context, value T) {
	b.mu.RLock()
	subscribers := b.m[reflect.TypeFor[T]()]
	b.mu.RUnlock()

	if len(subscribers) == 0 {
		panic(fmt.Sprintf("bus: no subscribers registered for type %T", value))
	}

	select {
	case b.sem <- struct{}{}:
	case <-ctx.Done():
		return
	}

	go func() {
		defer func() {
			<-b.sem
		}()

		for _, s := range subscribers {
			s.(Subscriber[T])(ctx, value)
		}
	}()
}

// Subscribe registers a subscriber for type T.
func (b *Bus) Subscribe[T any](subscriber Subscriber[T]) {
	if subscriber == nil {
		panic("bus: subscriber must not be nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	t := reflect.TypeFor[T]()
	b.m[t] = append(b.m[t], subscriber)
}

// Unsubscribe removes all subscribers registered for type T.
func (b *Bus) Unsubscribe[T any]() {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.m, reflect.TypeFor[T]())
}
