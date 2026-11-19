package gx_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shayanderson/gx"
)

// BenchmarkOverflowQueueBurst includes construction, overflow growth, and draining.
// Each operation processes 1024 items before allocating another queue.
func BenchmarkOverflowQueueBurst(b *testing.B) {
	benchmarkOverflowQueueOrders(b, func(b *testing.B, order gx.OverflowOrder) {
		const (
			items = 1024
			size  = 16
		)
		ctx := context.Background()
		b.ReportAllocs()
		for b.Loop() {
			q := gx.NewOverflowQueue(gx.OverflowQueueOptions[int]{
				Size:             size,
				Worker:           func(context.Context, int) error { return nil },
				OverflowCapacity: size,
				OverflowOrder:    order,
			})
			for range items {
				if !q.Push(1) {
					b.Fatal("burst push failed")
				}
			}
			q.Close()
			if err := q.Run(ctx); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(items, "items/op")
	})
}

func BenchmarkOverflowQueueMixedProducers(b *testing.B) {
	for _, waitProducers := range []int{0, 1, 4} {
		b.Run(
			fmt.Sprintf("push_%d/pushwait_%d", 8-waitProducers, waitProducers),
			func(b *testing.B) {
				benchmarkOverflowQueuePushMethods(b, 8, waitProducers)
			},
		)
	}
}

func BenchmarkOverflowQueuePushAccepted(b *testing.B) {
	benchmarkOverflowQueuePushAccepted(b, 1, false)
}

func BenchmarkOverflowQueuePushAcceptedParallel(b *testing.B) {
	benchmarkOverflowQueuePushAccepted(b, 4, true)
}

func BenchmarkOverflowQueuePushClosed(b *testing.B) {
	benchmarkOverflowQueueOrders(b, func(b *testing.B, order gx.OverflowOrder) {
		q := gx.NewOverflowQueue(gx.OverflowQueueOptions[int]{
			Size:          1,
			OverflowOrder: order,
		})
		q.Close()

		b.ReportAllocs()
		for b.Loop() {
			if q.Push(1) {
				b.Fatal("push to closed queue succeeded")
			}
		}
	})
}

func BenchmarkOverflowQueuePushFull(b *testing.B) {
	benchmarkOverflowQueueOrders(b, func(b *testing.B, order gx.OverflowOrder) {
		q := gx.NewOverflowQueue(gx.OverflowQueueOptions[int]{
			Size:          1,
			OverflowOrder: order,
			OverflowSize:  1,
		})
		defer q.Close()
		for range 2 {
			if !q.Push(1) {
				b.Fatal("initial push failed")
			}
		}

		b.ReportAllocs()
		for b.Loop() {
			if q.Push(1) {
				b.Fatal("push to full queue succeeded")
			}
		}
	})
}

func BenchmarkOverflowQueuePushWait(b *testing.B) {
	benchmarkOverflowQueuePushMethods(b, 1, 1)
}

func BenchmarkOverflowQueuePushWaitParallel(b *testing.B) {
	benchmarkOverflowQueuePushMethods(b, 8, 8)
}

func BenchmarkOverflowQueueThroughput(b *testing.B) {
	benchmarkOverflowQueueThroughput(b, false)
}

func BenchmarkOverflowQueueThroughputParallel(b *testing.B) {
	benchmarkOverflowQueueThroughput(b, true)
}

// benchmarkOverflowQueueCases varies order, worker count, and primary capacity.
func benchmarkOverflowQueueCases(
	b *testing.B,
	benchmark func(*testing.B, gx.OverflowQueueOptions[int]),
) {
	benchmarkOverflowQueueOrders(b, func(b *testing.B, order gx.OverflowOrder) {
		for _, workers := range []int{1, 4} {
			for _, size := range []int{1, 128, 4096} {
				b.Run(fmt.Sprintf("workers_%d/size_%d", workers, size), func(b *testing.B) {
					benchmark(b, gx.OverflowQueueOptions[int]{
						Size:          size,
						Workers:       workers,
						Worker:        func(context.Context, int) error { return nil },
						OverflowOrder: order,
						OverflowSize:  size,
					})
				})
			}
		}
	})
}

// benchmarkOverflowQueueOrders runs both overflow storage orders.
func benchmarkOverflowQueueOrders(b *testing.B, benchmark func(*testing.B, gx.OverflowOrder)) {
	for _, tt := range []struct {
		name  string
		order gx.OverflowOrder
	}{
		{"fifo", gx.OverflowFIFO},
		{"lifo", gx.OverflowLIFO},
	} {
		b.Run(tt.name, func(b *testing.B) { benchmark(b, tt.order) })
	}
}

// benchmarkOverflowQueuePushAccepted measures accepted pushes without overflow.
// Timing includes token handoff and worker receives, excluding final draining.
func benchmarkOverflowQueuePushAccepted(b *testing.B, workers int, parallel bool) {
	benchmarkOverflowQueueOrders(b, func(b *testing.B, order gx.OverflowOrder) {
		const size = 128

		slots := make(chan struct{}, size)
		for range size {
			slots <- struct{}{}
		}
		q := gx.NewOverflowQueue(gx.OverflowQueueOptions[int]{
			Size:    size,
			Workers: workers,
			Worker: func(context.Context, int) error {
				slots <- struct{}{}
				return nil
			},
			OverflowOrder: order,
			OverflowSize:  size,
		})
		done := make(chan error, 1)
		go func() { done <- q.Run(context.Background()) }()

		var failed atomic.Bool
		b.ReportAllocs()
		b.ResetTimer()
		if parallel {
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					<-slots
					if !q.Push(1) {
						failed.Store(true)
						return
					}
				}
			})
		} else {
			for b.Loop() {
				<-slots
				if !q.Push(1) {
					failed.Store(true)
					break
				}
			}
		}
		b.StopTimer()

		q.Close()
		if err := <-done; err != nil {
			b.Fatal(err)
		}
		if failed.Load() {
			b.Fatal("push with available capacity failed")
		}
	})
}

// benchmarkOverflowQueuePushMethods measures bounded queues through final draining.
// Producer time includes retries and scheduling but excludes final draining.
// It measures elapsed time per accepted Push, not call latency or aggregate throughput.
func benchmarkOverflowQueuePushMethods(b *testing.B, producers, waitProducers int) {
	benchmarkOverflowQueueCases(b, func(b *testing.B, opts gx.OverflowQueueOptions[int]) {
		ctx := context.Background()
		q := gx.NewOverflowQueue(opts)
		defer q.Close()
		done := make(chan error, 1)
		go func() { done <- q.Run(ctx) }()

		type result struct {
			elapsed time.Duration
			items   int
			retries int64
		}
		results := make([]result, producers)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for producer := range producers {
			items := b.N / producers
			if producer < b.N%producers {
				items++
			}
			wg.Go(func() {
				<-start
				if producer < waitProducers {
					for range items {
						if err := q.PushWait(ctx, 1); err != nil {
							b.Error(err)
							return
						}
					}
					return
				}

				var retries int64
				started := time.Now()
				for range items {
					for !q.Push(1) {
						retries++
						runtime.Gosched()
					}
				}
				results[producer] = result{
					elapsed: time.Since(started),
					items:   items,
					retries: retries,
				}
			})
		}

		b.ReportAllocs()
		b.ResetTimer()
		close(start)
		wg.Wait()
		q.Close()
		err := <-done
		b.StopTimer()
		if err != nil {
			b.Fatal(err)
		}

		var pushElapsed time.Duration
		var pushItems int
		var pushRetries int64
		for producer := waitProducers; producer < producers; producer++ {
			result := results[producer]
			pushElapsed += result.elapsed
			pushItems += result.items
			pushRetries += result.retries
		}
		if pushItems > 0 {
			b.ReportMetric(float64(pushRetries)/float64(pushItems), "retries/push")
			b.ReportMetric(
				float64(pushElapsed.Nanoseconds())/float64(pushItems),
				"push-producer-ns/item",
			)
		}
	})
}

// benchmarkOverflowQueueThroughput measures accepted pushes through final draining.
// Unlimited overflow may accumulate a growing backlog, including allocation costs.
// Compare it as burst absorption, not bounded steady-state throughput.
// retries/op counts rejected attempts per accepted item.
func benchmarkOverflowQueueThroughput(b *testing.B, parallel bool) {
	benchmarkOverflowQueueCases(b, func(b *testing.B, opts gx.OverflowQueueOptions[int]) {
		for _, limit := range []int{0, opts.OverflowSize} {
			b.Run(fmt.Sprintf("overflow_%d", limit), func(b *testing.B) {
				opts.OverflowSize = limit
				q := gx.NewOverflowQueue(opts)
				defer q.Close()
				done := make(chan error, 1)
				go func() { done <- q.Run(context.Background()) }()

				var retries atomic.Int64
				b.ReportAllocs()
				b.ResetTimer()
				if parallel {
					b.RunParallel(func(pb *testing.PB) {
						var localRetries int64
						for pb.Next() {
							for !q.Push(1) {
								localRetries++
								runtime.Gosched()
							}
						}
						retries.Add(localRetries)
					})
				} else {
					var localRetries int64
					for range b.N {
						for !q.Push(1) {
							localRetries++
							runtime.Gosched()
						}
					}
					retries.Store(localRetries)
				}
				q.Close()
				err := <-done
				b.StopTimer()
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(retries.Load())/float64(b.N), "retries/op")
			})
		}
	})
}
