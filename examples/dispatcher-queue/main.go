// Dispatcher-queue demonstrates buffering typed Dispatcher events with gx.Queue.
//
// Run with:
//
//	go run ./examples/dispatcher-queue
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/shayanderson/gx"
)

// Event gives Queue one concrete item type while preserving the concrete event
// type when the item is dispatched.
type Event interface {
	dispatch(context.Context, *gx.Dispatcher) error
}

type OrderPlaced struct {
	ID string
}

func (e OrderPlaced) dispatch(ctx context.Context, dispatcher *gx.Dispatcher) error {
	return dispatcher.Dispatch(ctx, e)
}

type OrderCanceled struct {
	ID string
}

func (e OrderCanceled) dispatch(ctx context.Context, dispatcher *gx.Dispatcher) error {
	return dispatcher.Dispatch(ctx, e)
}

func main() {
	dispatcher := gx.NewDispatcher()
	dispatcher.Register(func(_ context.Context, event OrderPlaced) error {
		fmt.Printf("audit: order placed %s\n", event.ID)
		return nil
	})
	dispatcher.Register(func(_ context.Context, event OrderCanceled) error {
		fmt.Printf("inventory: release reservation for %s\n", event.ID)
		return nil
	})

	queue := gx.NewQueue(gx.QueueOptions[Event]{
		// The buffer lets producers enqueue events without waiting for the
		// synchronous Dispatcher handlers to finish. Push returns false if this
		// buffer is full, so production code should choose a size and policy that
		// matches its backpressure requirements.
		Size: 16,

		// One worker preserves event order for this example. Increase Workers when
		// handlers are safe to run concurrently and ordering is not required.
		Workers: 1,

		Worker: func(ctx context.Context, event Event) error {
			// Dispatch through the concrete event value, allowing Dispatcher to
			// select only the handlers registered for that event's type.
			return event.dispatch(ctx, dispatcher)
		},
	})

	// Run the worker pool while producers enqueue events.
	runErr := make(chan error, 1)
	go func() { runErr <- queue.Run(context.Background()) }()

	for _, event := range []Event{
		OrderPlaced{ID: "order-123"},
		OrderCanceled{ID: "order-123"},
	} {
		if !queue.Push(event) {
			log.Fatal("event queue rejected an event")
		}
	}

	// Close drains the buffered events. Queue.Run returns after the worker has
	// dispatched every queued event.
	queue.Close()
	if err := <-runErr; err != nil {
		log.Fatal(err)
	}
}
