// Overflow-queue demonstrates ordering, push results, backpressure, and draining.
//
// Run with:
//
//	go run ./examples/overflow-queue
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/shayanderson/gx"
)

// main runs ordering, push result, and backpressure examples.
func main() {
	if err := orderingExample("FIFO", gx.OverflowFIFO); err != nil {
		log.Fatal(err)
	}
	if err := orderingExample("LIFO", gx.OverflowLIFO); err != nil {
		log.Fatal(err)
	}
	if err := tryPushExample(); err != nil {
		log.Fatal(err)
	}
	if err := backpressureExample(); err != nil {
		log.Fatal(err)
	}
}

// backpressureExample waits for capacity and drains accepted work before returning.
func backpressureExample() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gate := make(chan struct{})
	processed := make(chan int, 12)
	started := make(chan struct{})
	q := gx.NewOverflowQueue(gx.OverflowQueueOptions[int]{
		Size: 2,
		Worker: func(ctx context.Context, item int) error {
			if item == 1 {
				close(started)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-gate:
				}
			}

			// Simulate cancellation-aware I/O.
			timer := time.NewTimer(5 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				processed <- item
				return nil
			}
		},
		OverflowSize: 2,
	})
	defer q.Close()

	// Fill both stores before starting workers.
	for item := 1; item <= 4; item++ {
		if !q.Push(item) {
			return fmt.Errorf("initial push %d rejected", item)
		}
	}
	runErr := make(chan error, 1)
	go func() { runErr <- q.Run(ctx) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-started:
	}

	// Dequeuing alone does not free overflow capacity. The worker must finish.
	waitCtx, cancelWait := context.WithTimeout(ctx, 20*time.Millisecond)
	err := q.PushWait(waitCtx, 5)
	cancelWait()
	if !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("expected capacity timeout, got %v", err)
	}
	fmt.Printf("\nbackpressure: overflow=%d, PushWait timed out\n", q.Overflowed())

	close(gate)
	for item := 5; item <= 12; item++ {
		if err := q.PushWait(ctx, item); err != nil {
			return fmt.Errorf("push %d: %w", item, err)
		}
	}

	drainCtx, cancelDrain := context.WithTimeout(context.Background(), time.Second)
	defer cancelDrain()
	drainErr := q.Drain(drainCtx)
	if err := <-runErr; err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if drainErr != nil {
		return fmt.Errorf("drain: %w", drainErr)
	}

	close(processed)
	fmt.Print("drained:")
	for item := range processed {
		fmt.Printf(" %d", item)
	}
	fmt.Printf("\nclosed=%t overflow=%d\n", q.Closed(), q.Overflowed())
	return nil
}

// orderingExample separates primary FIFO order from the selected overflow order.
func orderingExample(name string, order gx.OverflowOrder) error {
	var processed []int
	q := gx.NewOverflowQueue(gx.OverflowQueueOptions[int]{
		Size: 2,
		Worker: func(_ context.Context, item int) error {
			processed = append(processed, item)
			return nil
		},
		OverflowOrder: order,
		OverflowSize:  4,
	})
	defer q.Close()

	for item := 1; item <= 6; item++ {
		if !q.Push(item) {
			return fmt.Errorf("push %d rejected", item)
		}
	}
	fmt.Printf("%s: overflow=%d, extra push accepted=%t\n", name, q.Overflowed(), q.Push(7))

	q.Close()
	if err := q.Run(context.Background()); err != nil {
		return err
	}
	fmt.Printf("processed: %v\n", processed)
	return nil
}

// tryPushExample reports placement before workers start consuming items.
func tryPushExample() error {
	q := gx.NewOverflowQueue(gx.OverflowQueueOptions[int]{
		Size: 1,
		Worker: func(_ context.Context, item int) error {
			fmt.Printf("processed: %d\n", item)
			return nil
		},
		OverflowSize: 1,
	})
	defer q.Close()

	fmt.Println("\nTryPush:")
	for item := 1; item <= 3; item++ {
		switch q.TryPush(item) {
		case gx.PushOverflowed:
			fmt.Printf("item %d: overflow\n", item)
		case gx.PushQueued:
			fmt.Printf("item %d: primary queue\n", item)
		case gx.PushRejected:
			fmt.Printf("item %d: rejected\n", item)
		}
	}

	q.Close()
	return q.Run(context.Background())
}
