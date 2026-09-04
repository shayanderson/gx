package gx_test

import (
	"context"
	"testing"
	"time"

	"github.com/shayanderson/gx"
	"github.com/shayanderson/gx/test"
)

func TestBusPublish(t *testing.T) {
	t.Parallel()

	b := gx.NewBus(1)
	values := make(chan string, 1)

	b.Subscribe(func(ctx context.Context, value string) {
		test.Equal(t, t.Context(), ctx)
		values <- value
	})

	b.Publish(t.Context(), "value")

	test.Equal(t, "value", receiveBusValue(t, values))
}

func TestBusDefaultMaxConcurrent(t *testing.T) {
	t.Parallel()

	b := gx.NewBus(0)
	values := make(chan string, 1)

	b.Subscribe(func(context.Context, string) {
		values <- "value"
	})

	b.Publish(t.Context(), "value")

	test.Equal(t, "value", receiveBusValue(t, values))
}

func TestBusMaxConcurrent(t *testing.T) {
	t.Parallel()

	b := gx.NewBus(1)

	started := make(chan string, 2)
	block := make(chan struct{})

	b.Subscribe(func(_ context.Context, value string) {
		started <- value
		<-block
	})

	b.Publish(t.Context(), "first")
	test.Equal(t, "first", receiveBusValue(t, started))

	go b.Publish(t.Context(), "second")

	select {
	case value := <-started:
		t.Fatalf("unexpected subscriber invocation: %v", value)
	case <-time.After(10 * time.Millisecond):
	}

	close(block)

	test.Equal(t, "second", receiveBusValue(t, started))
}

func TestBusPublishMultipleSubscribers(t *testing.T) {
	t.Parallel()

	b := gx.NewBus(1)
	values := make(chan string, 2)

	b.Subscribe(func(context.Context, string) {
		values <- "first"
	})
	b.Subscribe(func(context.Context, string) {
		values <- "second"
	})
	b.Subscribe(func(context.Context, int) {
		values <- "int"
	})

	b.Publish(t.Context(), "value")

	test.Equal(t, "first", receiveBusValue(t, values))
	test.Equal(t, "second", receiveBusValue(t, values))
}

func TestBusPublishNoSubscribersPanics(t *testing.T) {
	t.Parallel()

	b := gx.NewBus(1)

	test.Panics(t, func() {
		b.Publish(t.Context(), "missing")
	})
}

func TestBusSubscribeNilPanics(t *testing.T) {
	t.Parallel()

	b := gx.NewBus(1)

	test.Panics(t, func() {
		b.Subscribe[string](nil)
	})
}

func TestBusUnsubscribe(t *testing.T) {
	t.Parallel()

	b := gx.NewBus(1)
	values := make(chan string, 1)

	b.Subscribe(func(context.Context, string) {
		values <- "string"
	})
	b.Subscribe(func(context.Context, int) {
		values <- "int"
	})

	b.Unsubscribe[string]()

	b.Publish(t.Context(), 1)
	test.Equal(t, "int", receiveBusValue(t, values))

	test.Panics(t, func() {
		b.Publish(t.Context(), "missing")
	})
}

func TestBusPublishReturnsWhenContextDoneWaitingForSlot(t *testing.T) {
	t.Parallel()

	b := gx.NewBus(1)
	block := make(chan struct{})
	started := make(chan string, 2)

	b.Subscribe(func(_ context.Context, value string) {
		started <- value
		<-block
	})

	b.Publish(t.Context(), "first")
	test.Equal(t, "first", receiveBusValue(t, started))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	b.Publish(ctx, "second")

	select {
	case value := <-started:
		t.Fatalf("unexpected subscriber invocation: %v", value)
	default:
	}

	close(block)
}

func receiveBusValue[T any](t *testing.T, ch <-chan T) T {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for bus value")
	}

	var zero T
	return zero
}

func receiveBusSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for bus signal")
	}
}
