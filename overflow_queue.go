package gx

import (
	"context"
	"sync"
)

// overflowDefaultCapacity is the default initial overflow capacity.
const overflowDefaultCapacity = 64

// OverflowOrder controls overflow refill order.
type OverflowOrder int

const (
	// OverflowFIFO refills from the oldest overflowed items.
	OverflowFIFO OverflowOrder = iota

	// OverflowLIFO refills from the newest overflowed items.
	OverflowLIFO
)

// OverflowQueueOptions configures an OverflowQueue.
type OverflowQueueOptions[T any] struct {
	QueueOptions[T]

	// OverflowCapacity sets initial overflow capacity, capped by a positive OverflowSize.
	// Non-positive defaults to 64.
	OverflowCapacity int

	// OverflowOrder selects refill order. Values other than OverflowLIFO use FIFO.
	OverflowOrder OverflowOrder

	// OverflowSize limits overflow items. Non-positive means unlimited.
	OverflowSize int
}

// PushResult reports the outcome of TryPush.
type PushResult int

const (
	// PushRejected means the item was not accepted.
	PushRejected PushResult = iota

	// PushQueued means the item entered the primary queue.
	PushQueued

	// PushOverflowed means the item entered overflow.
	PushOverflowed
)

// overflowStore holds overflowed items.
type overflowStore[T any] interface {
	Len() int
	Push(T) bool
}

// OverflowQueue buffers excess items and refills the queue after each job.
// New items cannot bypass pending overflow. OverflowOrder controls refill order.
// Multiple workers do not guarantee processing order.
type OverflowQueue[T any] struct {
	closed     bool
	failOnFull bool
	limit      int
	mu         sync.Mutex
	overflow   overflowStore[T]
	queue      *Queue[T]
	ready      chan struct{}
	take       func() (T, bool)
}

// NewOverflowQueue creates a new OverflowQueue.
// FailOnFull applies only when the overflow limit is reached.
func NewOverflowQueue[T any](opts OverflowQueueOptions[T]) *OverflowQueue[T] {
	q := &OverflowQueue[T]{
		failOnFull: opts.FailOnFull,
		limit:      max(opts.OverflowSize, 0),
	}

	if w := opts.Worker; w != nil {
		opts.Worker = func(ctx context.Context, item T) error {
			defer q.drain()
			return w(ctx, item)
		}
	}
	opts.FailOnFull = false
	q.queue = NewQueue(opts.QueueOptions)

	size := opts.OverflowCapacity
	if size <= 0 {
		size = overflowDefaultCapacity
	}
	if q.limit > 0 {
		size = min(size, q.limit)
	}

	switch opts.OverflowOrder {
	case OverflowLIFO:
		s := NewStack[T](size)
		q.overflow, q.take = s, s.TryPop
	default:
		b := NewBuffer[T](size)
		q.overflow, q.take = b, b.TryNext
	}

	return q
}

// Close rejects new items and wakes blocked PushWait calls.
// The primary queue closes after overflow empties.
func (q *OverflowQueue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.closed = true
	q.notifyLocked()

	if q.overflow.Len() == 0 {
		q.queue.Close()
	}
}

// Closed reports whether the queue has been closed.
func (q *OverflowQueue[T]) Closed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.closed
}

// Drain closes the queue and waits for workers to exit.
// Cancellation stops workers and returns ctx.Err(), retaining queued items.
// It returns immediately if Run is inactive. Run reports worker errors.
func (q *OverflowQueue[T]) Drain(ctx context.Context) error {
	q.Close()

	queue := q.queue
	queue.mu.Lock()
	running, done, cancel := queue.running, queue.done, queue.cancel
	queue.mu.Unlock()

	if !running {
		return nil
	}

	select {
	case <-ctx.Done():
		cancel(context.Canceled)
		return ctx.Err()
	case <-done:
		return nil
	}
}

// Overflowed returns the overflow item count.
func (q *OverflowQueue[T]) Overflowed() int {
	return q.overflow.Len()
}

// Push enqueues an item, using overflow when full or overflow is pending.
// It returns false when closed or the overflow limit is reached.
func (q *OverflowQueue[T]) Push(item T) bool {
	return q.TryPush(item) != PushRejected
}

// PushWait enqueues an item, waiting if overflow is full.
// Capacity wakes waiters after worker completion, not dequeue.
// It returns ctx.Err() on cancellation or ErrQueueClosed on closure.
// Success means enqueued. It does not trigger FailOnFull.
func (q *OverflowQueue[T]) PushWait(ctx context.Context, item T) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return ErrQueueClosed
		}

		if q.pushLocked(item) != PushRejected {
			q.mu.Unlock()
			return nil
		}

		if q.ready == nil {
			q.ready = make(chan struct{})
		}
		ready := q.ready
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ready:
		}
	}
}

// Run processes items until the closed queue and overflow are drained, a
// worker fails, or ctx ends.
func (q *OverflowQueue[T]) Run(ctx context.Context) error {
	return q.queue.Run(ctx)
}

// RunUntilClosed ignores ctx cancellation and deadlines but preserves values.
// Workers stop on closure, Drain cancellation, or worker failure.
func (q *OverflowQueue[T]) RunUntilClosed(ctx context.Context) error {
	return q.queue.RunUntilClosed(ctx)
}

// TryPush enqueues without waiting and reports placement.
// It returns PushRejected when closed or the overflow limit is reached.
// FailOnFull applies only to rejection at the limit.
func (q *OverflowQueue[T]) TryPush(item T) PushResult {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return PushRejected
	}

	result := q.pushLocked(item)
	q.mu.Unlock()

	if result == PushRejected && q.failOnFull {
		q.queue.fail(ErrQueueFull)
	}
	return result
}

// drain refills available capacity and closes a closed queue once overflow empties.
func (q *OverflowQueue[T]) drain() {
	q.mu.Lock()
	defer q.mu.Unlock()

	ch := q.queue.queue
	moved := false

	// q.mu serializes sends and closure. Receivers only free capacity.
	for len(ch) < cap(ch) {
		item, ok := q.take()
		if !ok {
			break
		}
		q.queue.Push(item)
		moved = true
	}

	if moved {
		q.notifyLocked()
	}

	if q.closed && q.overflow.Len() == 0 {
		q.queue.Close()
	}
}

// notifyLocked wakes PushWait callers. Caller must hold q.mu.
func (q *OverflowQueue[T]) notifyLocked() {
	if q.ready != nil {
		close(q.ready)
		q.ready = nil
	}
}

// pushLocked enqueues without bypassing overflow and reports placement.
// Caller must hold q.mu.
func (q *OverflowQueue[T]) pushLocked(item T) PushResult {
	// Bypassing overflow is only safe when it is empty.
	if q.overflow.Len() == 0 && q.queue.Push(item) {
		return PushQueued
	}
	if q.limit > 0 && q.overflow.Len() >= q.limit {
		return PushRejected
	}
	if q.overflow.Push(item) {
		return PushOverflowed
	}
	return PushRejected
}
