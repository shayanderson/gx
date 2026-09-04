package gx

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestNewSemaphore(t *testing.T) {
	t.Parallel()

	s := NewSemaphore(2)

	test.Equal(t, 2, cap(s.ch))
}

func TestNewSemaphoreDefaultSize(t *testing.T) {
	t.Parallel()

	s := NewSemaphore(0)

	test.Equal(t, 1, cap(s.ch))
}

func TestSemaphoreAcquireAndRelease(t *testing.T) {
	t.Parallel()

	s := NewSemaphore(1)

	test.NoError(t, s.Acquire(t.Context()))
	test.False(t, s.TryAcquire())

	s.Release()

	test.True(t, s.TryAcquire())
	s.Release()
}

func TestSemaphoreAcquireContextCanceled(t *testing.T) {
	t.Parallel()

	s := NewSemaphore(1)
	test.NoError(t, s.Acquire(t.Context()))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := s.Acquire(ctx)

	test.True(t, errors.Is(err, context.Canceled))
	s.Release()
}

func TestSemaphoreAcquireBlocksUntilRelease(t *testing.T) {
	t.Parallel()

	s := NewSemaphore(1)
	test.NoError(t, s.Acquire(t.Context()))
	acquired := make(chan struct{})

	go func() {
		test.NoError(t, s.Acquire(t.Context()))
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("expected acquire to block")
	case <-time.After(10 * time.Millisecond):
	}

	s.Release()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for acquire")
	}

	s.Release()
}

func TestSemaphoreTryAcquire(t *testing.T) {
	t.Parallel()

	s := NewSemaphore(2)

	test.True(t, s.TryAcquire())
	test.True(t, s.TryAcquire())
	test.False(t, s.TryAcquire())

	s.Release()
	test.True(t, s.TryAcquire())
}
