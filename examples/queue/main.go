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
	"time"

	"github.com/shayanderson/gx"
)

type EmailJob struct {
	ID        int
	Recipient string
}

func main() {
	processed := make(chan string, 4)

	queue := gx.NewQueue(gx.QueueOptions[EmailJob]{
		// Size bounds the number of jobs waiting for a worker. Push returns
		// false instead of blocking if the queue is full or closed.
		Size: 4,

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

	// Run blocks until the queue is closed and drained, its context is canceled,
	// or a worker returns an error. Start it in a goroutine when producers should
	// continue adding work.
	runErr := make(chan error, 1)
	go func() { runErr <- queue.Run(context.Background()) }()

	for _, job := range []EmailJob{
		{ID: 1, Recipient: "ada@example.com"},
		{ID: 2, Recipient: "lin@example.com"},
		{ID: 3, Recipient: "margo@example.com"},
		{ID: 4, Recipient: "toni@example.com"},
	} {
		if !queue.Push(job) {
			log.Fatalf("could not queue email %d", job.ID)
		}
	}

	// Close prevents future Push calls, but workers finish every job already in
	// the queue. Once drained, Run returns nil.
	queue.Close()
	if err := <-runErr; err != nil {
		log.Fatal(err)
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

	pushWaitExample()
	drainExample()
}

// pushWaitExample shows how a producer can wait for capacity instead of
// handling a failed non-blocking Push call.
func pushWaitExample() {
	queue := gx.NewQueue(gx.QueueOptions[EmailJob]{
		Size: 1,
		Worker: func(_ context.Context, job EmailJob) error {
			fmt.Printf("queued with PushWait: email %d to %s\n", job.ID, job.Recipient)
			return nil
		},
	})

	runErr := make(chan error, 1)
	go func() { runErr <- queue.Run(context.Background()) }()

	if err := queue.PushWait(context.Background(), EmailJob{
		ID:        5,
		Recipient: "sam@example.com",
	}); err != nil {
		log.Fatal(err)
	}

	queue.Close()
	if err := <-runErr; err != nil {
		log.Fatal(err)
	}
}

// drainExample keeps workers alive during application shutdown, then gives
// queued and in-flight work a separate deadline to finish.
func drainExample() {
	appCtx, stopApp := context.WithCancel(context.Background())
	defer stopApp()

	started := make(chan struct{}, 2)
	queue := gx.NewQueue(gx.QueueOptions[EmailJob]{
		Size: 2,
		Worker: func(ctx context.Context, job EmailJob) error {
			started <- struct{}{}
			// Simulate cancellation-aware I/O using the worker context.
			timer := time.NewTimer(10 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				fmt.Printf("finished during drain: email %d to %s\n", job.ID, job.Recipient)
				return nil
			}
		},
	})

	runErr := make(chan error, 1)
	go func() { runErr <- queue.RunUntilClosed(appCtx) }()

	for _, job := range []EmailJob{
		{ID: 6, Recipient: "ash@example.com"},
		{ID: 7, Recipient: "jo@example.com"},
	} {
		if !queue.Push(job) {
			log.Fatalf("could not queue email %d", job.ID)
		}
	}
	<-started // Ensure Run is active before beginning shutdown.

	// RunUntilClosed keeps workers alive after application cancellation.
	stopApp()

	// Use a fresh context: appCtx is already canceled.
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), time.Second)
	defer cancelDrain()

	if err := queue.Drain(drainCtx); err != nil {
		log.Printf("queue drain interrupted: %v", err)
	}
	// Drain reports its own timeout/cancellation; Run reports worker errors.
	// On timeout, this wait still requires workers to honor cancellation.
	if err := <-runErr; err != nil {
		log.Printf("queue worker failed: %v", err)
	}
}
