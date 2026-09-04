package gx_test

import (
	"sync"
	"testing"
	"time"

	"github.com/shayanderson/gx"
	"github.com/shayanderson/gx/test"
)

func TestStackPushTryPop(t *testing.T) {
	t.Parallel()

	s := gx.NewStack[int](2)

	test.Equal(t, 0, s.Len())
	test.True(t, s.Push(1))
	test.True(t, s.Push(2))
	test.Equal(t, 2, s.Len())

	value, ok := s.TryPop()
	test.True(t, ok)
	test.Equal(t, 2, value)
	test.Equal(t, 1, s.Len())

	value, ok = s.TryPop()
	test.True(t, ok)
	test.Equal(t, 1, value)
	test.Equal(t, 0, s.Len())
}

func TestStackTryPopEmpty(t *testing.T) {
	t.Parallel()

	s := gx.NewStack[string](1)

	value, ok := s.TryPop()

	test.False(t, ok)
	test.Equal(t, "", value)
}

func TestStackGrowsAndPreservesOrder(t *testing.T) {
	t.Parallel()

	s := gx.NewStack[int](2)

	test.True(t, s.Push(1))
	test.True(t, s.Push(2))
	test.True(t, s.Push(3))
	test.True(t, s.Push(4))
	test.Equal(t, 4, s.Len())

	for _, expected := range []int{4, 3, 2, 1} {
		value, ok := s.TryPop()
		test.True(t, ok)
		test.Equal(t, expected, value)
	}

	test.Equal(t, 0, s.Len())
}

func TestStackPopBlocksUntilPush(t *testing.T) {
	t.Parallel()

	s := gx.NewStack[string](1)
	values := make(chan string, 1)

	go func() {
		value, ok := s.Pop()
		if ok {
			values <- value
		}
	}()

	select {
	case value := <-values:
		t.Fatalf("Pop returned before Push: %v", value)
	case <-time.After(10 * time.Millisecond):
	}

	test.True(t, s.Push("value"))
	test.Equal(t, "value", receiveStackValue(t, values))
}

func TestStackCloseDrainsValues(t *testing.T) {
	t.Parallel()

	s := gx.NewStack[int](1)

	test.True(t, s.Push(1))
	test.True(t, s.Push(2))
	s.Close()

	value, ok := s.Pop()
	test.True(t, ok)
	test.Equal(t, 2, value)

	value, ok = s.Pop()
	test.True(t, ok)
	test.Equal(t, 1, value)

	value, ok = s.Pop()
	test.False(t, ok)
	test.Equal(t, 0, value)
}

func TestStackCloseWakesPop(t *testing.T) {
	t.Parallel()

	s := gx.NewStack[int](1)
	done := make(chan bool, 1)

	go func() {
		_, ok := s.Pop()
		done <- ok
	}()

	s.Close()

	test.False(t, receiveStackValue(t, done))
}

func TestStackPushClosed(t *testing.T) {
	t.Parallel()

	s := gx.NewStack[int](0)

	s.Close()
	s.Close()

	test.False(t, s.Push(1))
	test.Equal(t, 0, s.Len())
}

func TestStackConcurrent(t *testing.T) {
	t.Parallel()

	s := gx.NewStack[int](1)

	const producers = 4
	const perProducer = 100

	var wg sync.WaitGroup

	for p := range producers {
		wg.Go(func() {
			for i := range perProducer {
				test.True(t, s.Push(p*perProducer+i))
			}
		})
	}

	go func() {
		wg.Wait()
		s.Close()
	}()

	count := 0
	for {
		_, ok := s.Pop()
		if !ok {
			break
		}
		count++
	}

	test.Equal(t, producers*perProducer, count)
}

func receiveStackValue[T any](t *testing.T, ch <-chan T) T {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stack value")
	}

	var zero T
	return zero
}
