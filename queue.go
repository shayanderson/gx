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

	// ErrQueueClosed is returned when trying to wait for a push to a closed queue.
	ErrQueueClosed = errors.New("queue is closed")

	// ErrQueueFull is returned when a queue configured to fail on full becomes full.
	ErrQueueFull = errors.New("queue is full")

	// ErrQueueWorkerRequired is returned when trying to run a queue without a worker.
	ErrQueueWorkerRequired = errors.New("worker must be provided")
)

// Worker processes an item from a Queue.
type Worker[T any] func(context.Context, T) error

// QueueOptions represents the options for creating a queue.
type QueueOptions[T any] struct {
	// FailOnFull causes Run to return ErrQueueFull if Push encounters a full queue.
	// It does not apply to PushWait.
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
	cancel      context.CancelCauseFunc
	closed      bool
	failOnFull  bool
	failure     error
	mu          sync.RWMutex
	pushNotify  chan struct{}
	pushWaiters atomic.Int64
	queue       chan T
	running     bool
	worker      Worker[T]
	workers     int
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
// Blocked and subsequent calls to PushWait return ErrQueueClosed unless canceled.
func (q *Queue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.closed = true
	close(q.queue)
	q.notifyPushWaitersLocked()
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
		q.mu.RUnlock()

		if q.failOnFull {
			q.fail(ErrQueueFull)
		}
		return false
	}
}

// PushWait waits for capacity, context cancellation, or queue closure.
// Returning nil means the item was enqueued, not processed.
// It returns ctx.Err() on cancellation or ErrQueueClosed on closure.
// Cancellation racing with an enqueue may still result in success.
// Unlike Push, encountering a full queue does not trigger FailOnFull.
// Run returning does not close the queue or wake blocked callers.
// Callers must cancel their context or call Close when processing stops.
func (q *Queue[T]) PushWait(ctx context.Context, item T) error {
	// Register before checking capacity so a dequeue cannot skip notification
	// between a failed send and waiting on pushNotify.
	q.pushWaiters.Add(1)
	defer q.pushWaiters.Add(-1)

	for {
		q.mu.Lock()
		if err := ctx.Err(); err != nil {
			q.mu.Unlock()
			return err
		}
		if q.closed {
			q.mu.Unlock()
			return ErrQueueClosed
		}

		select {
		case q.queue <- item:
			q.mu.Unlock()
			return nil
		default:
		}

		// Register under the same lock used to notify, avoiding missed wakeups.
		if q.pushNotify == nil {
			q.pushNotify = make(chan struct{})
		}
		notify := q.pushNotify
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-notify:
			// Another producer may have taken the available capacity, retry.
		}
	}
}

// Run starts processing items from the queue using the worker function.
// It blocks until the context is canceled or an error occurs in a worker.
func (q *Queue[T]) Run(ctx context.Context) error {
	if q.worker == nil {
		return ErrQueueWorkerRequired
	}

	parentCtx := ctx
	ctx, cancel := context.WithCancelCause(ctx)
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		cancel(nil)
		return ErrQueueAlreadyRunning
	}
	q.running = true
	if q.failure != nil {
		err := q.failure
		q.running = false
		q.mu.Unlock()
		cancel(nil)
		return err
	}
	q.cancel = cancel
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		q.cancel = nil
		q.running = false
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

					if q.pushWaiters.Load() > 0 {
						q.mu.Lock()
						q.notifyPushWaitersLocked()
						q.mu.Unlock()
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

	if errors.Is(parentCtx.Err(), context.Canceled) {
		return nil
	}

	err := context.Cause(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

// fail sets the queue's failure state, if unset, and cancels the context if the queue is running.
func (q *Queue[T]) fail(err error) {
	if err == nil {
		return
	}

	q.mu.Lock()
	if q.failure != nil {
		q.mu.Unlock()
		return
	}

	q.failure = err
	cancel := q.cancel
	q.mu.Unlock()

	if cancel != nil {
		cancel(err)
	}
}

// notifyPushWaitersLocked wakes blocked producers. The caller must hold mu exclusively.
func (q *Queue[T]) notifyPushWaitersLocked() {
	if q.pushNotify != nil {
		close(q.pushNotify)
		q.pushNotify = nil
	}
}
