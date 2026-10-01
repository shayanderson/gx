// Bus demonstrates typed, asynchronous event delivery with gx.Bus.
//
// Run with:
//
//	go run ./examples/bus
package main

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/shayanderson/gx"
)

type OrderPlaced struct {
	ID string
}

type OrderCanceled struct {
	ID string
}

func main() {
	// At most two published event deliveries may be active at once. Publish
	// waits for a slot before scheduling subscriber work, which can protect a
	// downstream dependency from too much concurrent work.
	bus := gx.NewBus(2)

	results := make(chan string, 3)
	var delivered sync.WaitGroup
	delivered.Add(3)

	// Subscribers are keyed by their concrete event type. Multiple subscribers
	// can receive the same event without the publisher knowing about them.
	bus.Subscribe(func(_ context.Context, event OrderPlaced) {
		defer delivered.Done()
		results <- "audit: order placed " + event.ID
	})
	bus.Subscribe(func(_ context.Context, event OrderPlaced) {
		defer delivered.Done()
		results <- "fulfillment: reserve inventory for " + event.ID
	})
	bus.Subscribe(func(_ context.Context, event OrderCanceled) {
		defer delivered.Done()
		results <- "fulfillment: release inventory for " + event.ID
	})

	// Publish starts subscriber delivery asynchronously. Pass a request or
	// application context so subscribers can stop work when the caller stops.
	bus.Publish(context.Background(), OrderPlaced{ID: "order-123"})
	bus.Publish(context.Background(), OrderCanceled{ID: "order-123"})

	// A long-running program usually continues naturally. This short example
	// waits so its asynchronous subscriber work completes before main returns.
	delivered.Wait()
	close(results)

	// Delivery from separate Publish calls can overlap, so sort only to keep the
	// demonstration's output stable; event-driven applications need not do this.
	lines := make([]string, 0, cap(results))
	for result := range results {
		lines = append(lines, result)
	}
	slices.Sort(lines)
	for _, line := range lines {
		fmt.Println(line)
	}
}
