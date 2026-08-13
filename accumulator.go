package gx

import (
	"context"
	"errors"
	"sync"
	"time"
)

// AccumulatorOptions configures an Accumulator.
type AccumulatorOptions struct {
	// Delay is the interval between time-based flushes.
	Delay time.Duration

	// Max is the accumulated value that triggers a flush. A value of zero
	// disables threshold-based flushing.
	Max int

	// Flush is called with the accumulated total when a flush occurs.
	Flush func(total int)
}

// Accumulator accumulates values and invokes a callback when either:
//   - the total reaches or exceeds max (if max > 0)
//   - each delay interval elapses and total > 0
//
// After either condition, the total is reset to zero.
// Zero totals are not flushed.
type Accumulator struct {
	closed bool
	delay  time.Duration
	fn     func(total int)
	max    int
	mu     sync.Mutex
	timer  *time.Timer
	total  int
}

// NewAccumulator creates a new Accumulator.
// A max of zero disables threshold-based flushing.
func NewAccumulator(ctx context.Context, opts AccumulatorOptions) (*Accumulator, error) {
	if opts.Delay <= 0 {
		return nil, errors.New("delay must be greater than zero")
	}
	if opts.Max < 0 {
		return nil, errors.New("max must not be negative")
	}
	if opts.Flush == nil {
		return nil, errors.New("fn must not be nil")
	}

	a := &Accumulator{
		delay: opts.Delay,
		fn:    opts.Flush,
		max:   opts.Max,
		timer: time.NewTimer(opts.Delay),
	}

	go a.run(ctx)

	return a, nil
}

// Add adds n to the accumulator.
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
	}
}

// Close flushes any remaining accumulated value and stops the accumulator.
// Close is safe to call multiple times.
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

// flushLocked invokes the callback and resets the accumulator.
// Caller must hold a.mu.
func (a *Accumulator) flushLocked() {
	if a.total == 0 {
		return
	}

	total := a.total
	a.total = 0

	a.fn(total)
}

// run handles delayed flushes and context cancellation.
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
			}
			a.timer.Reset(a.delay)

			a.mu.Unlock()
		}
	}
}
