// Fault demonstrates reporting the first fatal error from concurrent work.
//
// Run with:
//
//	go run ./examples/fault
package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shayanderson/gx"
)

var (
	errConfiguration = errors.New("configuration is invalid")
	errDependency    = errors.New("dependency is unavailable")
)

func main() {
	fault := gx.NewFault()

	// Done allows a coordinator to react as soon as any concurrent operation
	// reports a fatal error. Reading Err after Done closes safely returns the
	// recorded error.
	observed := make(chan struct{})
	go func() {
		<-fault.Done()
		fmt.Printf("coordinator: stopping new work: %v\n", fault.Err())
		close(observed)
	}()

	var workers sync.WaitGroup

	workers.Go(func() {
		fmt.Println("configuration: validating")
		if fault.Set(errConfiguration) {
			fmt.Println("configuration: recorded the fatal error")
		}
	})

	workers.Go(func() {
		// This operation succeeds, so it leaves Fault unchanged.
		time.Sleep(10 * time.Millisecond)
		fmt.Println("cache: warmed")
	})

	workers.Go(func() {
		// Later failures do not replace the first error. Set's return value is
		// useful when a caller needs to perform one-time failure handling.
		time.Sleep(20 * time.Millisecond)
		if !fault.Set(errDependency) {
			fmt.Println("dependency: fatal error already recorded")
		}
	})

	workers.Wait()
	<-observed

	// A Fault reports errors but does not cancel workers or wait for them. Use a
	// Runner when the first error should also cancel peer work through a context.
	fmt.Printf("final error: %v\n", fault.Err())
}
