package gx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var (
	// ErrQueueAlreadyRunning is returned when trying to run a queue that is already running.
	ErrQueueAlreadyRunning = errors.New("queue is already running")

	// ErrQueueFull is returned when a queue configured to fail on full becomes full.
	ErrQueueFull = errors.New("queue is full")

	// ErrQueueWorkerRequired is returned when trying to run a queue without a worker.
	ErrQueueWorkerRequired = errors.New("worker must be provided")
)

// Worker processes an item from a Queue.
type Worker[T any] func(context.Context, T) error

// QueueOptions represents the options for creating a queue.
type QueueOptions[T any] struct {
	// FailOnFull causes Run to return ErrQueueFull if the queue becomes full.
	FailOnFull bool

	// Size is the buffer size of the queue channel.
	Size int

	// Worker is the function that processes items from the queue.
	Worker Worker[T]

	// Workers is the number of worker goroutines to process items from the queue.
	Workers int
}

// Queue processes items using a pool of workers.
type Queue[T any] struct {
	cancel     context.CancelCauseFunc
	closed     bool
	failOnFull bool
	mu         sync.RWMutex
	queue      chan T
	running    atomic.Bool
	worker     Worker[T]
	workers    int
}

// NewQueue creates a new Queue with the specified number of workers,
// Queue buffer size and worker function.
// If workers is 0 or negative, it defaults to 1.
// If size is 0 or negative, it defaults to workers * 4.
func NewQueue[T any](opts QueueOptions[T]) *Queue[T] {
	if opts.Workers <= 0 {
		opts.Workers = 1
	}
	if opts.Size <= 0 {
		opts.Size = opts.Workers * 4
	}

	return &Queue[T]{
		failOnFull: opts.FailOnFull,
		workers:    opts.Workers,
		queue:      make(chan T, opts.Size),
		worker:     opts.Worker,
	}
}

// Close closes the queue and prevents new items from being added.
// Buffered items already in the queue are still processed.
// Subsequent calls to Push return false.
func (q *Queue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.closed = true
	close(q.queue)
}

// Closed reports whether the queue has been closed.
func (q *Queue[T]) Closed() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.closed
}

// Push adds an item to the queue.
// Returns false if the queue is full or closed.
func (q *Queue[T]) Push(item T) bool {
	q.mu.RLock()

	if q.closed {
		q.mu.RUnlock()
		return false
	}

	select {
	case q.queue <- item:
		q.mu.RUnlock()
		return true
	default:
		cancel := q.cancel
		failOnFull := q.failOnFull
		q.mu.RUnlock()

		if failOnFull && cancel != nil {
			cancel(ErrQueueFull)
		}
		return false
	}
}

// Run starts processing items from the queue using the worker function.
// It blocks until the context is canceled or an error occurs in a worker.
func (q *Queue[T]) Run(ctx context.Context) error {
	if q.worker == nil {
		return ErrQueueWorkerRequired
	}

	ctx, cancel := context.WithCancelCause(ctx)
	q.mu.Lock()
	if !q.running.CompareAndSwap(false, true) {
		q.mu.Unlock()
		cancel(nil)
		return ErrQueueAlreadyRunning
	}
	q.cancel = cancel
	q.mu.Unlock()
	defer func() {
		q.mu.Lock()
		q.cancel = nil
		q.running.Store(false)
		q.mu.Unlock()
		cancel(nil)
	}()

	var wg sync.WaitGroup

	for range q.workers {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return

				case item, ok := <-q.queue:
					if !ok {
						return
					}

					if err := q.worker(ctx, item); err != nil {
						cancel(err)
						return
					}
				}
			}
		})
	}

	wg.Wait()

	err := context.Cause(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
