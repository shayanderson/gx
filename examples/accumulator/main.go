// Accumulator demonstrates batching integer totals with gx.Accumulator.
//
// Run with:
//
//	go run ./examples/accumulator
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shayanderson/gx"
)

func main() {
	flushOnMax()
	flushOnDelayOrMax()
}

func flushOnMax() {
	fmt.Println("threshold-driven batch:")

	// Accumulator always needs a positive Delay. A long delay makes Max the
	// practical flush trigger for a batch-oriented workload, such as recording
	// metrics after every 10 events.
	a, err := gx.NewAccumulator(context.Background(), gx.AccumulatorOptions{
		Delay: time.Hour,
		Max:   10,
		Flush: func(total int) {
			fmt.Printf("  flushed %d events\n", total)
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	a.Add(3)
	a.Add(4)
	a.Add(3) // Total reaches 10, so Flush receives 10 immediately.

	// Close flushes a non-zero remainder and stops the accumulator. Always call
	// it when the producer is finished, even for a threshold-driven accumulator.
	a.Add(2)
	a.Close()
}

func flushOnDelayOrMax() {
	fmt.Println("\ntime-and-threshold-driven batch:")

	// This shape works well for periodic reporting: a quiet stream flushes on
	// the delay, while a busy stream flushes as soon as it reaches Max.
	a, err := gx.NewAccumulator(context.Background(), gx.AccumulatorOptions{
		Delay: 200 * time.Millisecond,
		Max:   10,
		Flush: func(total int) {
			fmt.Printf("  flushed %d events\n", total)
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// The total is below Max, so the next timer tick flushes 7.
	a.Add(3)
	a.Add(4)
	time.Sleep(250 * time.Millisecond)

	// This total reaches Max and flushes immediately; it does not wait for the
	// next delay interval.
	a.Add(6)
	a.Add(4)

	a.Close()
}
