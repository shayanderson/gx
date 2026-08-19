package gx_test

import (
	"context"
	"testing"

	"github.com/shayanderson/gx"
)

func BenchmarkDispatcherDispatch(b *testing.B) {
	type emptyEvent struct{}

	ctx := context.Background()
	d := gx.NewDispatcher()
	d.Register(func(context.Context, emptyEvent) error {
		return nil
	})
	event := emptyEvent{}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if err := d.Dispatch(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDispatcherDispatchParallel(b *testing.B) {
	type emptyEvent struct{}

	ctx := context.Background()
	d := gx.NewDispatcher()
	d.Register(func(context.Context, emptyEvent) error {
		return nil
	})

	event := emptyEvent{}

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := d.Dispatch(ctx, event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkDispatcherDispatchMultipleHandlers(b *testing.B) {
	type emptyEvent struct{}

	ctx := context.Background()
	d := gx.NewDispatcher()
	d.Register(func(context.Context, emptyEvent) error { return nil })
	d.Register(func(context.Context, emptyEvent) error { return nil })
	d.Register(func(context.Context, emptyEvent) error { return nil })
	event := emptyEvent{}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if err := d.Dispatch(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDispatcherDispatchMultipleTypes(b *testing.B) {
	type eventA struct{}
	type eventB struct{}

	ctx := context.Background()
	d := gx.NewDispatcher()
	d.Register(func(context.Context, eventA) error { return nil })
	d.Register(func(context.Context, eventB) error { return nil })

	a := eventA{}
	e := eventB{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		if i%2 == 0 {
			if err := d.Dispatch(ctx, a); err != nil {
				b.Fatal(err)
			}
			continue
		}

		if err := d.Dispatch(ctx, e); err != nil {
			b.Fatal(err)
		}
	}
}
