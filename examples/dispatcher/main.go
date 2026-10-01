// Dispatcher demonstrates synchronous, typed event handling with gx.Dispatcher.
//
// Run with:
//
//	go run ./examples/dispatcher
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/shayanderson/gx"
)

var errOutOfStock = errors.New("product is out of stock")

type OrderSubmitted struct {
	ID      string
	Product string
}

func main() {
	dispatcher := gx.NewDispatcher()

	// Handlers are registered per concrete event type and execute in
	// registration order in the caller's goroutine.
	dispatcher.Register(func(_ context.Context, event OrderSubmitted) error {
		fmt.Printf("  audit: received order %s\n", event.ID)
		return nil
	})
	dispatcher.Register(func(_ context.Context, event OrderSubmitted) error {
		if event.Product == "unavailable" {
			return errOutOfStock
		}
		fmt.Printf("  inventory: reserved %s\n", event.Product)
		return nil
	})
	dispatcher.Register(func(_ context.Context, event OrderSubmitted) error {
		fmt.Printf("  email: sent confirmation for %s\n", event.ID)
		return nil
	})

	ctx := context.Background()

	fmt.Println("successful dispatch:")
	if err := dispatcher.Dispatch(ctx, OrderSubmitted{
		ID:      "order-123",
		Product: "keyboard",
	}); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nfailed dispatch:")
	err := dispatcher.Dispatch(ctx, OrderSubmitted{
		ID:      "order-124",
		Product: "unavailable",
	})
	if errors.Is(err, errOutOfStock) {
		// Dispatch returns the handler error immediately. Handlers registered
		// after the failing one do not run, so no confirmation email is sent.
		fmt.Printf("  caller received error: %v\n", err)
	}
}
