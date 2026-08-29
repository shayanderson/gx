package gx

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestThrottler_Allow_FirstCallAllowed(t *testing.T) {
	t.Parallel()
	th := NewThrottler(100 * time.Millisecond)
	test.True(t, th.Allow())
}

func TestThrottler_Allow_NonPositiveIntervalDisablesThrottling(t *testing.T) {
	t.Parallel()

	for _, interval := range []time.Duration{0, -time.Second} {
		th := NewThrottler(interval)
		test.True(t, th.Allow())
		test.True(t, th.Allow())
	}
}

func TestThrottler_Allow_ThrottledWithinInterval(t *testing.T) {
	t.Parallel()
	th := NewThrottler(200 * time.Millisecond)
	test.True(t, th.Allow())
	test.False(t, th.Allow())
}

func TestThrottler_Allow_AfterInterval(t *testing.T) {
	t.Parallel()
	th := NewThrottler(5 * time.Millisecond)

	test.True(t, th.Allow())
	time.Sleep(10 * time.Millisecond)
	test.True(t, th.Allow())
}

func TestThrottler_Do(t *testing.T) {
	t.Parallel()
	th := NewThrottler(5 * time.Millisecond)

	var c atomic.Int32

	test.True(t, th.Do(func() {
		c.Add(1)
	}))
	test.Equal(t, int32(1), c.Load())

	test.False(t, th.Do(func() {
		c.Add(1)
	}))
	test.Equal(t, int32(1), c.Load())

	time.Sleep(10 * time.Millisecond)

	test.True(t, th.Do(func() {
		c.Add(1)
	}))
	test.Equal(t, int32(2), c.Load())
}

func TestThrottler_ConcurrentAllow(t *testing.T) {
	t.Parallel()
	th := NewThrottler(200 * time.Millisecond)

	const goroutines = 10

	var wg sync.WaitGroup
	var allowed atomic.Int32

	wg.Add(goroutines)
	for range goroutines {
		wg.Go(func() {
			defer wg.Done()
			if th.Allow() {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()

	test.Equal(t, int32(1), allowed.Load())
}

func TestThrottler_RaceSafety(t *testing.T) {
	t.Parallel()
	th := NewThrottler(10 * time.Millisecond)

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		wg.Go(func() {
			defer wg.Done()
			for range iterations {
				th.Allow()
				time.Sleep(time.Millisecond)
			}
		})
	}

	wg.Wait()
}
