package gx

import (
	"context"
	"testing"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestNewAccumulator_InvalidArgs(t *testing.T) {
	ctx := t.Context()

	_, err := NewAccumulator(ctx, 0, 1, func(int) {})
	test.NotNil(t, err)

	_, err = NewAccumulator(ctx, time.Second, -1, func(int) {})
	test.NotNil(t, err)

	_, err = NewAccumulator(ctx, time.Second, 1, nil)
	test.NotNil(t, err)

	_, err = NewAccumulator(ctx, time.Second, 0, nil)
	test.NotNil(t, err)
}

func TestAccumulator_Max(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var total int

	a, err := NewAccumulator(
		ctx,
		time.Hour,
		10,
		func(n int) {
			total = n
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
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ch := make(chan int, 1)

	a, err := NewAccumulator(
		ctx,
		50*time.Millisecond,
		100,
		func(n int) {
			ch <- n
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
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var total int

	a, err := NewAccumulator(
		ctx,
		time.Hour,
		100,
		func(n int) {
			total = n
		},
	)
	test.NoError(t, err)

	a.Add(2)
	a.Add(5)

	a.Close()

	test.Equal(t, 7, total)
}

func TestAccumulator_CloseEmpty(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0

	a, err := NewAccumulator(
		ctx,
		time.Hour,
		100,
		func(int) {
			calls++
		},
	)
	test.NoError(t, err)

	a.Close()

	test.Equal(t, 0, calls)
}

func TestAccumulator_CloseTwice(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0

	a, err := NewAccumulator(
		ctx,
		time.Hour,
		100,
		func(int) {
			calls++
		},
	)
	test.NoError(t, err)

	a.Add(5)

	a.Close()
	a.Close()

	test.Equal(t, 1, calls)
}

func TestAccumulator_AddAfterClose(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0

	a, err := NewAccumulator(
		ctx,
		time.Hour,
		10,
		func(n int) {
			calls++
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
	a := &Accumulator{
		fn: func(int) {
			t.Fatal("should not be called")
		},
	}

	a.mu.Lock()
	a.flushLocked()
	a.mu.Unlock()
}

func TestAccumulator_TimerResets(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ch := make(chan int, 1)

	a, err := NewAccumulator(
		ctx,
		50*time.Millisecond,
		100,
		func(n int) {
			ch <- n
		},
	)
	test.NoError(t, err)

	a.Add(3)

	// wait for half the delay, then add again
	// this should restart the timer
	time.Sleep(25 * time.Millisecond)

	a.Add(4)

	// ensure the callback does NOT happen at the original 50ms mark
	select {
	case n := <-ch:
		t.Fatalf("callback fired too early with %d", n)

	case <-time.After(35 * time.Millisecond):
		// good: callback has not fired yet
	}

	// it should fire after the delay from the second Add()
	select {
	case n := <-ch:
		test.Equal(t, 7, n)

	case <-time.After(time.Second):
		t.Fatal("timeout waiting for callback")
	}
}

func TestAccumulator_NoMax(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ch := make(chan int, 1)

	a, err := NewAccumulator(
		ctx,
		50*time.Millisecond,
		0, // no threshold flushing
		func(n int) {
			ch <- n
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
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var total int

	a, err := NewAccumulator(
		ctx,
		time.Hour,
		0,
		func(n int) {
			total = n
		},
	)
	test.NoError(t, err)

	a.Add(2)
	a.Add(3)

	a.Close()

	test.Equal(t, 5, total)
}

func TestAccumulator_EmptyTimerReset(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0

	_, err := NewAccumulator(
		ctx,
		10*time.Millisecond,
		0,
		func(int) {
			calls++
		},
	)
	test.NoError(t, err)

	// let several timer expirations happen with total == 0
	time.Sleep(50 * time.Millisecond)

	test.Equal(t, 0, calls)
}

func TestAccumulator_CloseAfterTimerFired(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan struct{}, 1)

	a, err := NewAccumulator(
		ctx,
		10*time.Millisecond,
		0,
		func(int) {
			done <- struct{}{}
		},
	)
	test.NoError(t, err)

	a.Add(1)

	<-done

	// timer has already fired and been drained by run()
	a.Close()
}
