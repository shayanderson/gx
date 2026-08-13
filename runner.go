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

// Run runs a function and handles errors.
// It sets the first error to the app error.
func (g *Runner) Run(fn func() error) {
	g.wg.Go(func() {
		if err := fn(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel(err)
				}
			})
		}
	})
}

// Wait blocks until all app goroutines are done.
// It returns the first error if it exists.
func (g *Runner) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel(g.err)
	}
	return g.err
}
