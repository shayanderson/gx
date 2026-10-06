// Queue demonstrates concurrent job processing with gx.Queue.
//
// Run with:
//
//	go run ./examples/queue
package main

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/shayanderson/gx"
)

type EmailJob struct {
	ID        int
	Recipient string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := []EmailJob{
		{ID: 1, Recipient: "ada@example.com"},
		{ID: 2, Recipient: "lin@example.com"},
		{ID: 3, Recipient: "margo@example.com"},
		{ID: 4, Recipient: "toni@example.com"},
	}
	processed := make(chan string, len(jobs))

	queue := gx.NewQueue(gx.QueueOptions[EmailJob]{
		// Size bounds the number of jobs waiting for a worker.
		Size: 1,

		// Workers controls the number of jobs processed concurrently.
		Workers: 2,

		Worker: func(ctx context.Context, job EmailJob) error {
			// Real work should use ctx for cancellation-aware I/O.
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			processed <- fmt.Sprintf("sent email %d to %s", job.ID, job.Recipient)
			return nil
		},
	})

	// Push tries immediately and returns false if the queue is full or closed.
	// This first job fills the buffer before workers start.
	if !queue.Push(jobs[0]) {
		log.Fatal("could not queue first email")
	}

	// Run blocks until the queue is closed and drained, its context is canceled,
	// or a worker returns an error. Start it in a goroutine when producers should
	// continue adding work.
	runErr := make(chan error, 1)
	go func() {
		err := queue.Run(ctx)
		// Run returning does not close the queue. Cancel producers so a worker
		// error cannot leave them blocked in PushWait.
		cancel()
		runErr <- err
	}()

	var pushErr error
	for _, job := range jobs[1:] {
		// PushWait waits for capacity, ctx cancellation, or queue closure.
		// A nil error means enqueued, not processed.
		if err := queue.PushWait(ctx, job); err != nil {
			pushErr = fmt.Errorf("queue email %d: %w", job.ID, err)
			break
		}
	}

	// Close prevents new pushes. On the successful path, leave ctx active so
	// workers finish every queued job; Run returns nil once the queue is drained.
	queue.Close()
	if err := <-runErr; err != nil {
		log.Fatal(err)
	}
	if pushErr != nil {
		log.Fatal(pushErr)
	}

	close(processed)
	lines := make([]string, 0, cap(processed))
	for result := range processed {
		lines = append(lines, result)
	}
	// Worker completion order is intentionally nondeterministic. Sorting keeps
	// the example's output stable without changing Queue's concurrency.
	slices.Sort(lines)
	for _, line := range lines {
		fmt.Println(line)
	}
}
