// Throttler demonstrates rate-limiting an action with gx.Throttler.
//
// Run with:
//
//	go run ./examples/throttler
package main

import (
	"fmt"
	"time"

	"github.com/shayanderson/gx"
)

func main() {
	throttleWithDo()
	throttleWithAllow()
}

func throttleWithDo() {
	fmt.Println("Do:")

	throttler := gx.NewThrottler(100 * time.Millisecond)
	for request := 1; request <= 3; request++ {
		run := throttler.Do(func() {
			fmt.Printf("  refreshed for request %d\n", request)
		})
		fmt.Printf("  request %d ran: %t\n", request, run)
	}

	// The interval starts when the action is allowed, not when it finishes.
	time.Sleep(120 * time.Millisecond)
	throttler.Do(func() { fmt.Println("  refreshed after the interval") })
}

func throttleWithAllow() {
	fmt.Println("\nAllow:")

	throttler := gx.NewThrottler(100 * time.Millisecond)
	if throttler.Allow() {
		fmt.Println("  sent a status update")
	}
	if !throttler.Allow() {
		// Allow is useful when the action has branching or return values that do
		// not fit naturally inside Do's callback.
		fmt.Println("  skipped a duplicate status update")
	}

	time.Sleep(120 * time.Millisecond)
	if throttler.Allow() {
		fmt.Println("  sent the next status update")
	}
}
