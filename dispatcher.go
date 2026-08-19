package gx

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Handler is a function that handles a value of type T.
type Handler[T any] func(context.Context, T) error

// Dispatcher dispatches typed values to registered handlers.
type Dispatcher struct {
	m map[reflect.Type][]any
	mu sync.RWMutex
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		m: make(map[reflect.Type][]any),
	}
}

// Dispatch dispatches value to all handlers registered for type T.
// If a handler returns an error, dispatching stops and the error is returned.
func (d *Dispatcher) Dispatch[T any](ctx context.Context, value T) error {
	d.mu.RLock()
	handlers := d.m[reflect.TypeFor[T]()]
	d.mu.RUnlock()

	if len(handlers) == 0 {
		return fmt.Errorf("dispatcher: no handler registered for type %T", value)
	}

	for _, h := range handlers {
		if err := h.(Handler[T])(ctx, value); err != nil {
			return err
		}
	}

	return nil
}

// Register registers a handler for type T.
func (d *Dispatcher) Register[T any](handler Handler[T]) {
	if handler == nil {
		panic("dispatcher: handler must not be nil")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	t := reflect.TypeFor[T]()
	d.m[t] = append(d.m[t], handler)
}

// Unregister removes all handlers registered for type T.
func (d *Dispatcher) Unregister[T any]() {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.m, reflect.TypeFor[T]())
}