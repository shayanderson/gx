package gx

import (
	"sync"
	"time"
)

// Debouncer delays execution until no new calls occur within the
// configured interval.
type Debouncer struct {
	delay time.Duration
	mu    sync.Mutex
	timer *time.Timer
}

// NewDebouncer returns a new debouncer. If delay is less than or equal
// to zero, a delay of 10 milliseconds is used.
func NewDebouncer(delay time.Duration) *Debouncer {
	if delay <= 0 {
		delay = 10 * time.Millisecond
	}

	return &Debouncer{
		delay: delay,
	}
}

// Cancel cancels any pending function execution. If the function has
// already started executing, Cancel has no effect.
func (d *Debouncer) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}

// Do schedules fn to run after the configured delay. If called again
// before the delay expires, the pending execution is canceled and the
// delay restarts.
func (d *Debouncer) Do(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.delay, fn)
}
