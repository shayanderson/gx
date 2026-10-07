// WaitQueue demonstrates cancellable producer backpressure with gx.WaitQueue.
//
// Run with:
//
//	go run ./examples/wait-queue
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

	queue := gx.NewWaitQueue(gx.WaitQueueOptions[EmailJob]{
		// Size bounds jobs waiting for a worker. PushWait blocks when it is full.
		Size: 1,

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

	// If Run stops because a worker fails, cancel producers so they do not remain
	// blocked in PushWait.
	runErr := make(chan error, 1)
	go func() {
		err := queue.Run(ctx)
		cancel()
		runErr <- err
	}()

	for _, job := range jobs {
		// A nil error means job was enqueued, not that the email was sent.
		if err := queue.PushWait(ctx, job); err != nil {
			queue.Close()
			if runErr := <-runErr; runErr != nil {
				log.Fatal(runErr)
			}
			log.Fatalf("could not queue email %d: %v", job.ID, err)
		}
	}

	// Close prevents future Push and PushWait calls, but lets workers drain work.
	queue.Close()
	if err := <-runErr; err != nil {
		log.Fatal(err)
	}

	close(processed)
	lines := make([]string, 0, cap(processed))
	for result := range processed {
		lines = append(lines, result)
	}
	slices.Sort(lines)
	for _, line := range lines {
		fmt.Println(line)
	}
}
