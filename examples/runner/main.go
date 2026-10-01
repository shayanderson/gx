// Runner demonstrates coordinating concurrent tasks with gx.Runner.
//
// Run with:
//
//	go run ./examples/runner
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/shayanderson/gx"
)

var errConfigurationMissing = errors.New("configuration is missing")

func main() {
	runner, ctx := gx.NewRunner(context.Background())
	workerReady := make(chan struct{})

	runner.Run(func() error {
		fmt.Println("background task: waiting for cancellation")
		close(workerReady)

		// Every task should observe the context returned by NewRunner. When any
		// task returns an error, Runner cancels it with that error as its cause.
		<-ctx.Done()
		fmt.Printf("background task: stopped (%v)\n", context.Cause(ctx))
		return nil
	})

	runner.Run(func() error {
		<-workerReady
		fmt.Println("startup task: configuration validation failed")
		return errConfigurationMissing
	})

	// Wait does not return until every task exits. It returns the first task
	// error, while the derived context carries the same cancellation cause.
	err := runner.Wait()
	if !errors.Is(err, errConfigurationMissing) {
		log.Fatal(err)
	}
	fmt.Printf("runner returned: %v\n", err)
}
