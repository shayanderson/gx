package gx_test

import (
	"context"
	"sync"
	"testing"

	"github.com/shayanderson/gx"
)

func BenchmarkBusPublish(b *testing.B) {
	type emptyEvent struct{}

	ctx := context.Background()
	bus := gx.NewBus(1)
	event := emptyEvent{}

	var wg sync.WaitGroup
	bus.Subscribe(func(context.Context, emptyEvent) {
		wg.Done()
	})

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		wg.Add(1)
		bus.Publish(ctx, event)
	}

	b.StopTimer()
	wg.Wait()
}

func BenchmarkBusPublishMultipleSubscribers(b *testing.B) {
	type emptyEvent struct{}

	ctx := context.Background()
	bus := gx.NewBus(1)
	bus.Subscribe(func(context.Context, emptyEvent) {})
	bus.Subscribe(func(context.Context, emptyEvent) {})
	bus.Subscribe(func(context.Context, emptyEvent) {})
	event := emptyEvent{}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		bus.Publish(ctx, event)
	}
}

func BenchmarkBusPublishMultipleTypes(b *testing.B) {
	type eventA struct{}
	type eventB struct{}

	ctx := context.Background()
	bus := gx.NewBus(1)
	bus.Subscribe(func(context.Context, eventA) {})
	bus.Subscribe(func(context.Context, eventB) {})

	a := eventA{}
	e := eventB{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		if i%2 == 0 {
			bus.Publish(ctx, a)
			continue
		}

		bus.Publish(ctx, e)
	}
}

func BenchmarkBusPublishParallel(b *testing.B) {
	type emptyEvent struct{}

	ctx := context.Background()
	bus := gx.NewBus(16)
	bus.Subscribe(func(context.Context, emptyEvent) {})
	event := emptyEvent{}

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bus.Publish(ctx, event)
		}
	})
}
