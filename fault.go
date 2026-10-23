package gx

import "sync"

// Fault records the first fatal asynchronous error.
// The zero value is ready to use.
type Fault struct {
	done     chan struct{}
	err      error
	initOnce sync.Once
	once     sync.Once
}

// NewFault creates a new Fault.
func NewFault() *Fault {
	return new(Fault)
}

// Done returns a channel that closes when a fatal error is recorded.
func (f *Fault) Done() <-chan struct{} {
	f.init()
	return f.done
}

// Err returns the recorded fatal error, or nil if none has been recorded.
func (f *Fault) Err() error {
	f.init()

	select {
	case <-f.done:
		return f.err
	default:
		return nil
	}
}

// Set records the first non-nil error and closes Done. It reports whether err
// was recorded. Calls after the first recorded error have no effect.
func (f *Fault) Set(err error) bool {
	if err == nil {
		return false
	}

	f.init()

	set := false
	f.once.Do(func() {
		f.err = err
		close(f.done)
		set = true
	})

	return set
}

// init creates the completion channel on first use.
func (f *Fault) init() {
	f.initOnce.Do(func() {
		f.done = make(chan struct{})
	})
}
