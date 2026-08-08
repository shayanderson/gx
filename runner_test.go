package gx

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestRunner(t *testing.T) {
	runner, ctx := NewRunner(t.Context())

	done := make(chan struct{})
	runner.Run(func() error {
		close(done)
		return nil
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not finish")
	}

	select {
	case <-ctx.Done():
		t.Fatal("context canceled before Wait")
	default:
	}

	test.NoError(t, runner.Wait())
}

func TestRunnerError(t *testing.T) {
	runner, ctx := NewRunner(t.Context())

	errFirst := errors.New("first error")
	errSecond := errors.New("second error")

	runner.Run(func() error {
		return errFirst
	})
	runner.Run(func() error {
		time.Sleep(10 * time.Millisecond)
		return errSecond
	})

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("context was not canceled after first error")
	}

	test.Error(t, runner.Wait(), errFirst)

	test.Error(t, context.Cause(ctx), errFirst)
}
