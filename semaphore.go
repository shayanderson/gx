package gx

import "context"

// Semaphore limits concurrent access to a resource.
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore returns a new semaphore with the specified capacity.
// If size is less than or equal to zero, a capacity of one is used.
func NewSemaphore(size int) *Semaphore {
	if size <= 0 {
		size = 1
	}

	return &Semaphore{
		ch: make(chan struct{}, size),
	}
}

// Acquire acquires the semaphore, blocking until a slot is available or
// the context is canceled. On success, it returns nil. On failure, it
// returns ctx.Err() and leaves the semaphore unchanged.
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases the semaphore.
func (s *Semaphore) Release() {
	<-s.ch
}

// TryAcquire attempts to acquire the semaphore without blocking. It
// returns true if successful or false if no slot is available.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true

	default:
		return false
	}
}
