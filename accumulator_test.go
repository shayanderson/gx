package gx

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestNewAccumulator_InvalidArgs(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	_, err := NewAccumulator(ctx, AccumulatorOptions{Max: 1, Flush: func(int) {}})
	test.NotNil(t, err)

	_, err = NewAccumulator(
		ctx,
		AccumulatorOptions{Delay: time.Second, Max: -1, Flush: func(int) {}},
	)
	test.NotNil(t, err)

	_, err = NewAccumulator(ctx, AccumulatorOptions{Delay: time.Second, Max: 1})
	test.NotNil(t, err)

	_, err = NewAccumulator(ctx, AccumulatorOptions{Delay: time.Second})
	test.NotNil(t, err)
}

func TestAccumulator_Max(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var total int

	a, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: time.Hour,
			Max:   10,
			Flush: func(n int) {
				total = n
			},
		},
	)
	test.NoError(t, err)

	a.Add(3)
	a.Add(4)

	test.Equal(t, 0, total)

	a.Add(3)

	test.Equal(t, 10, total)
}

func TestAccumulator_Delay(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ch := make(chan int, 1)

	a, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: 50 * time.Millisecond,
			Max:   100,
			Flush: func(n int) {
				ch <- n
			},
		},
	)
	test.NoError(t, err)

	a.Add(3)
	a.Add(4)

	select {
	case n := <-ch:
		test.Equal(t, 7, n)

	case <-time.After(time.Second):
		t.Fatal("timeout waiting for callback")
	}
}

func TestAccumulator_Close(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var total int

	a, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: time.Hour,
			Max:   100,
			Flush: func(n int) {
				total = n
			},
		},
	)
	test.NoError(t, err)

	a.Add(2)
	a.Add(5)

	a.Close()

	test.Equal(t, 7, total)
}

func TestAccumulator_CloseEmpty(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0

	a, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: time.Hour,
			Max:   100,
			Flush: func(int) {
				calls++
			},
		},
	)
	test.NoError(t, err)

	a.Close()

	test.Equal(t, 0, calls)
}

func TestAccumulator_CloseTwice(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0

	a, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: time.Hour,
			Max:   100,
			Flush: func(int) {
				calls++
			},
		},
	)
	test.NoError(t, err)

	a.Add(5)

	a.Close()
	a.Close()

	test.Equal(t, 1, calls)
}

func TestAccumulator_AddAfterClose(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0

	a, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: time.Hour,
			Max:   10,
			Flush: func(n int) {
				calls++
			},
		},
	)
	test.NoError(t, err)

	a.Add(5)
	a.Close()

	test.Equal(t, 1, calls)

	a.Add(5)

	test.Equal(t, 1, calls)
}

func TestAccumulator_FlushLockedZero(t *testing.T) {
	t.Parallel()

	a := &Accumulator{
		fn: func(int) {
			t.Fatal("should not be called")
		},
	}

	a.mu.Lock()
	a.flushLocked()
	a.mu.Unlock()
}

func TestAccumulator_TimerDoesNotReset(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ch := make(chan int, 1)

	a, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: 50 * time.Millisecond,
			Max:   100,
			Flush: func(n int) {
				ch <- n
			},
		},
	)
	test.NoError(t, err)

	a.Add(3)

	// wait for half the delay, then add again.
	// this should not restart the timer.
	time.Sleep(25 * time.Millisecond)

	a.Add(4)

	// it should fire on the original interval, not delay from the second Add.
	select {
	case n := <-ch:
		test.Equal(t, 7, n)

	case <-time.After(time.Second):
		t.Fatal("timeout waiting for callback")
	}
}

func TestAccumulator_NoMax(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ch := make(chan int, 1)

	a, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: 50 * time.Millisecond,
			Max:   0, // no threshold flushing
			Flush: func(n int) {
				ch <- n
			},
		},
	)
	test.NoError(t, err)

	a.Add(3)
	a.Add(4)
	a.Add(5)

	// should not flush immediately
	select {
	case n := <-ch:
		t.Fatalf("callback fired too early with %d", n)

	default:
	}

	// should flush after delay
	select {
	case n := <-ch:
		test.Equal(t, 12, n)

	case <-time.After(time.Second):
		t.Fatal("timeout waiting for callback")
	}
}

func TestAccumulator_NoMaxClose(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var total int

	a, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: time.Hour,
			Max:   0,
			Flush: func(n int) {
				total = n
			},
		},
	)
	test.NoError(t, err)

	a.Add(2)
	a.Add(3)

	a.Close()

	test.Equal(t, 5, total)
}

func TestAccumulator_EmptyTimerReset(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0

	_, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: 10 * time.Millisecond,
			Max:   0,
			Flush: func(int) {
				calls++
			},
		},
	)
	test.NoError(t, err)

	// let several timer expirations happen with total == 0
	time.Sleep(50 * time.Millisecond)

	test.Equal(t, 0, calls)
}

func TestAccumulator_CloseAfterTimerFired(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan struct{}, 1)

	a, err := NewAccumulator(
		ctx,
		AccumulatorOptions{
			Delay: 10 * time.Millisecond,
			Max:   0,
			Flush: func(int) {
				done <- struct{}{}
			},
		},
	)
	test.NoError(t, err)

	a.Add(1)

	<-done

	// timer has already fired and been drained by run()
	a.Close()
}

// TestAccumulator_CloseStopsRunGoroutine guards against Close leaving run()'s
// goroutine alive. Uses context.Background() on purpose: if Close doesn't
// stop run() on its own, nothing else here ever will.
func TestAccumulator_CloseStopsRunGoroutine(t *testing.T) {
	before := runtime.NumGoroutine()

	a, err := NewAccumulator(context.Background(), AccumulatorOptions{
		Delay: time.Millisecond,
		Max:   100,
		Flush: func(int) {},
	})
	test.NoError(t, err)

	a.Close()

	// give run()'s goroutine a moment to see ctx.Done() and return
	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	test.LessOrEqual(t, runtime.NumGoroutine(), before)
}
