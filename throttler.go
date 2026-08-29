package gx

import (
	"sync"
	"time"
)

// Throttler limits how often an action may run by allowing
// at most one execution per interval.
type Throttler struct {
	interval time.Duration
	lastRun  time.Time
	mu       sync.Mutex
}

// NewThrottler creates a Throttler that allows at most one execution per
// interval. A non-positive interval disables throttling.
func NewThrottler(interval time.Duration) *Throttler {
	return &Throttler{interval: interval}
}

// Allow reports whether an action may run now.
// It returns true if the caller is considered to have "used" the interval.
func (t *Throttler) Allow() bool {
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.lastRun.IsZero() && now.Sub(t.lastRun) < t.interval {
		return false
	}
	t.lastRun = now
	return true
}

// Do executes the provided function if allowed by the throttler.
// It returns true if the function was executed.
func (t *Throttler) Do(fn func()) bool {
	if t.Allow() {
		fn()
		return true
	}
	return false
}
