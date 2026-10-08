// Retry demonstrates retrying a transient operation with gx.Retry.
//
// Run with:
//
//	go run ./examples/retry
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/shayanderson/gx"
)

var errServiceUnavailable = errors.New("service unavailable")

func main() {
	retry, err := gx.NewRetry(gx.RetryOptions{
		// Attempts includes the first call. Retry returns the last operation
		// error if every attempt fails.
		Attempts: 5,

		// Delay is the wait after the first failure. Backoff multiplies it for
		// later failures, while MaxDelay caps the wait at 100ms.
		Delay:    50 * time.Millisecond,
		Backoff:  2,
		Jitter:   0.5,
		MaxDelay: 100 * time.Millisecond,

		// MaxDuration applies a deadline to the retry context and bounds the
		// entire operation, including retry waits.
		MaxDuration: time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

	attempt := 0
	err = retry.Do(context.Background(), func(ctx context.Context) error {
		attempt++
		fmt.Printf("attempt %d\n", attempt)

		// A real operation should pass ctx to its network or database call so it
		// honors cancellation and MaxDuration.
		if err := callService(ctx, attempt); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("request succeeded after %d attempts\n", attempt)
}

func callService(ctx context.Context, attempt int) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Simulate a dependency that recovers after three temporary failures.
	if attempt < 4 {
		return errServiceUnavailable
	}
	return nil
}
