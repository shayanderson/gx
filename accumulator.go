package gx

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Accumulator accumulates values and invokes a callback when either:
//   - the total reaches or exceeds max (if max > 0)
//   - delay elapses and total > 0
//
// after either condition, the total is reset to zero
type Accumulator struct {
	closed bool
	delay  time.Duration
	fn     func(total int)
	max    int
	mu     sync.Mutex
	timer  *time.Timer
	total  int
}

// NewAccumulator creates a new Accumulator
//
// a max of zero disables threshold-based flushing
func NewAccumulator(
	ctx context.Context,
	delay time.Duration,
	max int,
	fn func(total int),
) (*Accumulator, error) {
	if delay <= 0 {
		return nil, errors.New("delay must be greater than zero")
	}
	if max < 0 {
		return nil, errors.New("max must not be negative")
	}
	if fn == nil {
		return nil, errors.New("fn must not be nil")
	}

	a := &Accumulator{
		delay: delay,
		fn:    fn,
		max:   max,
		timer: time.NewTimer(delay),
	}

	go a.run(ctx)

	return a, nil
}

// Add adds n to the accumulator
func (a *Accumulator) Add(n int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return
	}

	a.total += n

	// flush on threshold if enabled
	if a.max > 0 && a.total >= a.max {
		a.flushLocked()
		return
	}

	// restart timer
	if !a.timer.Stop() {
		select {
		case <-a.timer.C:
		default:
		}
	}
	a.timer.Reset(a.delay)
}

// Close flushes any remaining accumulated value and stops the accumulator
// Close is safe to call multiple times
func (a *Accumulator) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return
	}
	a.closed = true

	// stop timer and drain channel if needed
	if !a.timer.Stop() {
		select {
		case <-a.timer.C:
		default:
		}
	}

	if a.total > 0 {
		total := a.total
		a.total = 0
		a.fn(total)
	}
}

// flushLocked invokes the callback and resets the accumulator
// caller must hold a.mu
func (a *Accumulator) flushLocked() {
	if a.total == 0 {
		return
	}

	total := a.total
	a.total = 0

	a.fn(total)
}

// run handles delayed flushes and context cancellation
func (a *Accumulator) run(ctx context.Context) {
	defer a.timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-a.timer.C:
			a.mu.Lock()

			if a.total > 0 {
				a.flushLocked()
			} else {
				a.timer.Reset(a.delay)
			}

			a.mu.Unlock()
		}
	}
}
