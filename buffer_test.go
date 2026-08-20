package gx_test

import (
	"sync"
	"testing"
	"time"

	"github.com/shayanderson/gx"
	"github.com/shayanderson/gx/test"
)

func TestBufferPushTryNext(t *testing.T) {
	b := gx.NewBuffer[int](2)

	test.Equal(t, 0, b.Len())
	test.True(t, b.Push(1))
	test.True(t, b.Push(2))
	test.Equal(t, 2, b.Len())

	value, ok := b.TryNext()
	test.True(t, ok)
	test.Equal(t, 1, value)
	test.Equal(t, 1, b.Len())

	value, ok = b.TryNext()
	test.True(t, ok)
	test.Equal(t, 2, value)
	test.Equal(t, 0, b.Len())
}

func TestBufferTryNextEmpty(t *testing.T) {
	b := gx.NewBuffer[string](1)

	value, ok := b.TryNext()

	test.False(t, ok)
	test.Equal(t, "", value)
}

func TestBufferGrowsAndPreservesOrder(t *testing.T) {
	b := gx.NewBuffer[int](2)

	test.True(t, b.Push(1))
	test.True(t, b.Push(2))

	value, ok := b.TryNext()
	test.True(t, ok)
	test.Equal(t, 1, value)

	test.True(t, b.Push(3))
	test.True(t, b.Push(4))
	test.True(t, b.Push(5))
	test.Equal(t, 4, b.Len())

	for _, expected := range []int{2, 3, 4, 5} {
		value, ok := b.TryNext()
		test.True(t, ok)
		test.Equal(t, expected, value)
	}

	test.Equal(t, 0, b.Len())
}

func TestBufferNextBlocksUntilPush(t *testing.T) {
	b := gx.NewBuffer[string](1)
	values := make(chan string, 1)

	go func() {
		value, ok := b.Next()
		if ok {
			values <- value
		}
	}()

	select {
	case value := <-values:
		t.Fatalf("Next returned before Push: %v", value)
	case <-time.After(10 * time.Millisecond):
	}

	test.True(t, b.Push("value"))
	test.Equal(t, "value", receiveBufferValue(t, values))
}

func TestBufferCloseDrainsBufferedValues(t *testing.T) {
	b := gx.NewBuffer[int](1)

	test.True(t, b.Push(1))
	test.True(t, b.Push(2))
	b.Close()

	value, ok := b.Next()
	test.True(t, ok)
	test.Equal(t, 1, value)

	value, ok = b.Next()
	test.True(t, ok)
	test.Equal(t, 2, value)

	value, ok = b.Next()
	test.False(t, ok)
	test.Equal(t, 0, value)
}

func TestBufferCloseWakesNext(t *testing.T) {
	b := gx.NewBuffer[int](1)
	done := make(chan bool, 1)

	go func() {
		_, ok := b.Next()
		done <- ok
	}()

	b.Close()

	test.False(t, receiveBufferValue(t, done))
}

func TestBufferPushClosed(t *testing.T) {
	b := gx.NewBuffer[int](0)

	b.Close()
	b.Close()

	test.False(t, b.Push(1))
	test.Equal(t, 0, b.Len())
}

func receiveBufferValue[T any](t *testing.T, ch <-chan T) T {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for buffer value")
	}

	var zero T
	return zero
}

func TestBufferConcurrent(t *testing.T) {
	b := gx.NewBuffer[int](1)

	const producers = 4
	const perProducer = 100

	var wg sync.WaitGroup

	for p := range producers {
		wg.Go(func() {
			for i := range perProducer {
				test.True(t, b.Push(p*perProducer+i))
			}
		})
	}

	go func() {
		wg.Wait()
		b.Close()
	}()

	count := 0
	for {
		_, ok := b.Next()
		if !ok {
			break
		}
		count++
	}

	test.Equal(t, producers*perProducer, count)
}
