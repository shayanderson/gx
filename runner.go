package gx

import (
	"context"
	"sync"
)

// Runner is a task runner.
type Runner struct {
	cancel  func(error)
	err     error
	errOnce sync.Once
	wg      sync.WaitGroup
}

// NewRunner creates a new Runner.
func NewRunner(ctx context.Context) (*Runner, context.Context) {
	ctx, cancel := context.WithCancelCause(ctx)
	return &Runner{cancel: cancel}, ctx
}

// Cancel records err as the runner's first error and cancels its context.
//
// Subsequent calls have no effect. err must be non-nil.
func (r *Runner) Cancel(err error) {
	if err == nil {
		panic("gx: Runner.Cancel requires a non-nil error")
	}

	r.errOnce.Do(func() {
		r.err = err
		r.cancel(err)
	})
}

// Run runs a function and handles errors.
// It sets the first error to the app error.
func (r *Runner) Run(fn func() error) {
	r.wg.Go(func() {
		if err := fn(); err != nil {
			r.errOnce.Do(func() {
				r.err = err
				if r.cancel != nil {
					r.cancel(err)
				}
			})
		}
	})
}

// Wait blocks until all app goroutines are done.
// It returns the first error if it exists.
func (r *Runner) Wait() error {
	r.wg.Wait()
	if r.cancel != nil {
		r.cancel(r.err)
	}
	return r.err
}
