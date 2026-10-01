// Debouncer demonstrates delaying work until a burst of activity stops.
//
// Run with:
//
//	go run ./examples/debouncer
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/shayanderson/gx"
)

func main() {
	searchAfterTypingStops()
	cancelPendingWork()
}

func searchAfterTypingStops() {
	fmt.Println("typing a search query:")

	const delay = 100 * time.Millisecond
	debouncer := gx.NewDebouncer(delay)
	searches := make(chan string, 1)

	// Each keystroke restarts the delay. Only the final function runs after
	// there have been no calls to Do for delay.
	for _, query := range []string{"g", "gx", "gx runtime"} {
		debouncer.Do(func() {
			searches <- query
		})
		time.Sleep(30 * time.Millisecond)
	}

	select {
	case query := <-searches:
		fmt.Printf("  searching for %q\n", query)
	case <-time.After(time.Second):
		log.Fatal("timed out waiting for debounced search")
	}
}

func cancelPendingWork() {
	fmt.Println("\ncanceling pending work:")

	debouncer := gx.NewDebouncer(100 * time.Millisecond)
	ran := make(chan struct{}, 1)
	debouncer.Do(func() { ran <- struct{}{} })

	// Cancel prevents a pending callback from running. It has no effect once a
	// callback has already started.
	debouncer.Cancel()

	select {
	case <-ran:
		log.Fatal("canceled callback ran")
	case <-time.After(150 * time.Millisecond):
		fmt.Println("  callback did not run")
	}
}
