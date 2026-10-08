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

func BenchmarkQueuePushFull(b *testing.B) {
	q := gx.NewQueue(gx.QueueOptions[int]{Size: 1})
	defer q.Close()
	if !q.Push(1) {
		b.Fatal("initial push failed")
	}

	b.ReportAllocs()
	for b.Loop() {
		if q.Push(1) {
			b.Fatal("push to full queue succeeded")
		}
	}
}

func BenchmarkQueuePushClosed(b *testing.B) {
	q := gx.NewQueue(gx.QueueOptions[int]{Size: 1})
	q.Close()

	b.ReportAllocs()
	for b.Loop() {
		if q.Push(1) {
			b.Fatal("push to closed queue succeeded")
		}
	}
}

func BenchmarkQueuePushAccepted(b *testing.B) {
	benchmarkQueuePushAccepted(b, 1, false)
}

func BenchmarkQueuePushAcceptedParallel(b *testing.B) {
	benchmarkQueuePushAccepted(b, 4, true)
}

func BenchmarkQueueThroughput(b *testing.B) {
	benchmarkQueueThroughput(b, false)
}

func BenchmarkQueueThroughputParallel(b *testing.B) {
	benchmarkQueueThroughput(b, true)
}

func BenchmarkQueuePushWait(b *testing.B) {
	benchmarkQueuePushMethods(b, 1, 1)
}

func BenchmarkQueuePushWaitParallel(b *testing.B) {
	benchmarkQueuePushMethods(b, 8, 8)
}

func BenchmarkQueueMixedProducers(b *testing.B) {
	// Keep the total producer count fixed for an apples-to-apples comparison.
	for _, waitProducers := range []int{0, 1, 4} {
		b.Run(
			fmt.Sprintf("push_%d/pushwait_%d", 8-waitProducers, waitProducers),
			func(b *testing.B) {
				benchmarkQueuePushMethods(b, 8, waitProducers)
			},
		)
	}
}

// Each operation successfully enqueues one item without retrying. slots tracks
// available queue capacity, which the worker returns as soon as it dequeues an
// item. Timing includes that capacity handoff and the worker receive path.
func benchmarkQueuePushAccepted(b *testing.B, workers int, parallel bool) {
	const size = 128

	slots := make(chan struct{}, size)
	for range size {
		slots <- struct{}{}
	}
	q := gx.NewQueue(gx.QueueOptions[int]{
		Size:    size,
		Workers: workers,
		Worker: func(context.Context, int) error {
			slots <- struct{}{}
			return nil
		},
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
	if failed.Load() {
		b.Fatal("push with available capacity failed")
	}
	if err := <-done; err != nil {
		b.Fatal(err)
	}
}

// Each operation successfully enqueues one item. Timing includes retries and
// draining all accepted items through Run, including its worker receive path.
// A no-op worker exposes queue overhead. retries/op reports capacity contention,
// rejected pushes are never counted as completed operations.
func benchmarkQueueThroughput(b *testing.B, parallel bool) {
	for _, workers := range []int{1, 4} {
		for _, size := range []int{1, 128, 4096} {
			b.Run(fmt.Sprintf("workers_%d/size_%d", workers, size), func(b *testing.B) {
				q := gx.NewQueue(gx.QueueOptions[int]{
					Size:    size,
					Workers: workers,
					Worker:  func(context.Context, int) error { return nil },
				})
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
	}
}

// Each producer uses one method for its entire share of b.N accepted items.
// ns/op includes final draining. push-producer-ns/item measures elapsed producer
// time per accepted Push item, including retries and scheduling, but not final
// draining. It is averaged across Push producers, not aggregate throughput or
// individual non-blocking-call latency. Clocks are read only once per batch.
func benchmarkQueuePushMethods(b *testing.B, producers, waitProducers int) {
	for _, workers := range []int{1, 4} {
		for _, size := range []int{1, 128, 4096} {
			b.Run(fmt.Sprintf("workers_%d/size_%d", workers, size), func(b *testing.B) {
				ctx := context.Background()
				q := gx.NewQueue(gx.QueueOptions[int]{
					Size:    size,
					Workers: workers,
					Worker:  func(context.Context, int) error { return nil },
				})
				defer q.Close()
				done := make(chan error, 1)
				go func() { done <- q.Run(ctx) }()

				type result struct {
					items   int
					retries int64
					elapsed time.Duration
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
						results[producer] = result{items, retries, time.Since(started)}
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

				var pushItems int
				var pushRetries int64
				var pushElapsed time.Duration
				for producer := waitProducers; producer < producers; producer++ {
					result := results[producer]
					pushItems += result.items
					pushRetries += result.retries
					pushElapsed += result.elapsed
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
	}
}
