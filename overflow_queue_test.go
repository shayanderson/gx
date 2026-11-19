package gx

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestOverflowQueueOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		order    OverflowOrder
		expected []int
	}{
		{"fifo", OverflowFIFO, []int{1, 2, 3, 4, 5, 6}},
		{"lifo", OverflowLIFO, []int{1, 2, 6, 5, 4, 3}},
		{"unknown", OverflowOrder(99), []int{1, 2, 3, 4, 5, 6}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var processed []int
			q := NewOverflowQueue(OverflowQueueOptions[int]{
				Size: 2,
				Worker: func(_ context.Context, item int) error {
					processed = append(processed, item)
					return nil
				},
				OverflowOrder: tt.order,
			})

			for i := 1; i <= 6; i++ {
				test.True(t, q.Push(i))
			}
			test.Equal(t, 4, q.Overflowed())

			q.Close()
			test.True(t, q.Closed())
			test.NoError(t, q.Run(t.Context()))
			test.Equal(t, tt.expected, processed)
			test.Equal(t, 0, q.Overflowed())
		})
	}
}

func TestOverflowQueueNoBypass(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		gate := make(chan struct{})
		var processed []int
		q := NewOverflowQueue(OverflowQueueOptions[int]{
			Size: 1,
			Worker: func(_ context.Context, item int) error {
				<-gate
				processed = append(processed, item)
				return nil
			},
		})
		runErr := make(chan error, 1)
		go func() { runErr <- q.RunUntilClosed(t.Context()) }()

		test.True(t, q.Push(1))
		synctest.Wait()
		test.True(t, q.Push(2))
		test.True(t, q.Push(3))
		test.Equal(t, 1, q.Overflowed())

		// Item 2 is now in flight, leaving the channel empty while 3 is still in overflow.
		gate <- struct{}{}
		synctest.Wait()
		test.Equal(t, 0, len(q.queue.queue))
		test.Equal(t, 1, q.Overflowed())

		test.True(t, q.Push(4))
		test.Equal(t, 2, q.Overflowed())

		close(gate)
		q.Close()
		test.NoError(t, <-runErr)
		test.Equal(t, []int{1, 2, 3, 4}, processed)
	})
}

func TestOverflowQueueLimit(t *testing.T) {
	t.Parallel()

	for _, failOnFull := range []bool{false, true} {
		name := "reject"
		if failOnFull {
			name = "fail on full"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var processed []int
			q := NewOverflowQueue(OverflowQueueOptions[int]{
				FailOnFull: failOnFull,
				Size:       1,
				Worker: func(_ context.Context, item int) error {
					processed = append(processed, item)
					return nil
				},
				OverflowSize: 2,
			})

			test.True(t, q.Push(1))
			test.True(t, q.Push(2))
			test.True(t, q.Push(3))
			test.False(t, q.Push(4))
			test.Equal(t, 2, q.Overflowed())

			q.Close()
			err := q.Run(t.Context())
			if failOnFull {
				test.ErrorIs(t, err, ErrQueueFull)
				test.Len(t, 0, processed)
				return
			}
			test.NoError(t, err)
			test.Equal(t, []int{1, 2, 3}, processed)
		})
	}
}

func TestOverflowQueueFailOnFullRunning(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		q := NewOverflowQueue(OverflowQueueOptions[int]{
			FailOnFull: true,
			Size:       1,
			Worker: func(ctx context.Context, _ int) error {
				<-ctx.Done()
				return ctx.Err()
			},
			OverflowSize: 1,
		})
		runErr := make(chan error, 1)
		go func() { runErr <- q.Run(t.Context()) }()

		test.True(t, q.Push(1))
		synctest.Wait()
		test.True(t, q.Push(2))
		test.True(t, q.Push(3))
		test.Equal(t, 1, q.Overflowed())
		test.False(t, q.Push(4))
		test.ErrorIs(t, <-runErr, ErrQueueFull)
		q.Close()
	})
}

func TestOverflowQueueUnlimited(t *testing.T) {
	t.Parallel()

	for _, size := range []int{0, -1} {
		q := NewOverflowQueue(OverflowQueueOptions[int]{
			Size:         1,
			OverflowSize: size,
		})
		for i := range 100 {
			test.True(t, q.Push(i))
		}
		test.Equal(t, 99, q.Overflowed())
	}
}

func TestOverflowQueuePushWait(t *testing.T) {
	t.Parallel()

	for _, failOnFull := range []bool{false, true} {
		name := "reject"
		if failOnFull {
			name = "fail on full"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				var processed []int
				q := NewOverflowQueue(OverflowQueueOptions[int]{
					FailOnFull: failOnFull,
					Size:       1,
					Worker: func(_ context.Context, item int) error {
						processed = append(processed, item)
						return nil
					},
					OverflowSize: 1,
				})
				test.True(t, q.Push(1))
				test.True(t, q.Push(2))

				pushErr := make(chan error, 1)
				go func() { pushErr <- q.PushWait(t.Context(), 3) }()
				synctest.Wait()
				test.Equal(t, 0, len(pushErr))

				runErr := make(chan error, 1)
				go func() { runErr <- q.RunUntilClosed(t.Context()) }()
				test.NoError(t, <-pushErr)

				q.Close()
				test.NoError(t, <-runErr)
				test.Equal(t, []int{1, 2, 3}, processed)
			})
		})
	}
}

func TestOverflowQueuePushWaitUnblock(t *testing.T) {
	t.Parallel()

	for _, closeQueue := range []bool{false, true} {
		name := "cancel"
		if closeQueue {
			name = "close"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				q := NewOverflowQueue(OverflowQueueOptions[int]{
					Size:         1,
					OverflowSize: 1,
				})
				test.True(t, q.Push(1))
				test.True(t, q.Push(2))

				pushErr := make(chan error, 1)
				go func() { pushErr <- q.PushWait(ctx, 3) }()
				synctest.Wait()
				test.Equal(t, 0, len(pushErr))

				if closeQueue {
					q.Close()
					test.ErrorIs(t, <-pushErr, ErrQueueClosed)
				} else {
					cancel()
					test.ErrorIs(t, <-pushErr, context.Canceled)
				}
				test.Equal(t, 1, q.Overflowed())
			})
		})
	}
}

func TestOverflowQueueClose(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		gate := make(chan struct{})
		var processed []int
		q := NewOverflowQueue(OverflowQueueOptions[int]{
			Size: 1,
			Worker: func(_ context.Context, item int) error {
				<-gate
				processed = append(processed, item)
				return nil
			},
		})
		runErr := make(chan error, 1)
		go func() { runErr <- q.RunUntilClosed(t.Context()) }()

		test.True(t, q.Push(1))
		synctest.Wait()
		test.True(t, q.Push(2))
		test.True(t, q.Push(3))
		test.Equal(t, 1, q.Overflowed())

		q.Close()
		q.Close()
		test.True(t, q.Closed())
		test.False(t, q.Push(4))
		test.ErrorIs(t, q.PushWait(t.Context(), 4), ErrQueueClosed)

		synctest.Wait()
		test.Equal(t, 0, len(runErr))

		close(gate)
		test.NoError(t, <-runErr)
		test.Equal(t, []int{1, 2, 3}, processed)
	})
}

func TestOverflowQueuePushWaitCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	q := NewOverflowQueue(OverflowQueueOptions[int]{})
	test.ErrorIs(t, q.PushWait(ctx, 1), context.Canceled)
	test.Equal(t, 0, len(q.queue.queue))
}

func TestOverflowQueueDrainNotRunning(t *testing.T) {
	t.Parallel()

	var processed []int
	q := NewOverflowQueue(OverflowQueueOptions[int]{
		Size: 1,
		Worker: func(_ context.Context, item int) error {
			processed = append(processed, item)
			return nil
		},
	})
	test.True(t, q.Push(1))
	test.True(t, q.Push(2))
	test.True(t, q.Push(3))

	test.NoError(t, q.Drain(t.Context()))
	test.True(t, q.Closed())
	test.Equal(t, 2, q.Overflowed())

	test.NoError(t, q.Run(t.Context()))
	test.Equal(t, []int{1, 2, 3}, processed)
}

func TestOverflowQueueDrain(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		gate := make(chan struct{})
		var processed []int
		q := NewOverflowQueue(OverflowQueueOptions[int]{
			Size: 1,
			Worker: func(_ context.Context, item int) error {
				<-gate
				processed = append(processed, item)
				return nil
			},
		})
		runErr := make(chan error, 1)
		go func() { runErr <- q.RunUntilClosed(t.Context()) }()

		test.True(t, q.Push(1))
		synctest.Wait()
		test.True(t, q.Push(2))
		test.True(t, q.Push(3))

		drainErr := make(chan error, 1)
		go func() { drainErr <- q.Drain(t.Context()) }()
		synctest.Wait()
		test.Equal(t, 0, len(drainErr))

		close(gate)
		test.NoError(t, <-drainErr)
		test.NoError(t, <-runErr)
		test.Equal(t, []int{1, 2, 3}, processed)
	})
}

func TestOverflowQueueDrainTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		gate := make(chan struct{})
		var processed []int
		q := NewOverflowQueue(OverflowQueueOptions[int]{
			Size: 1,
			Worker: func(ctx context.Context, item int) error {
				select {
				case <-gate:
				case <-ctx.Done():
					return ctx.Err()
				}
				processed = append(processed, item)
				return nil
			},
		})
		test.True(t, q.Push(1))
		test.True(t, q.Push(2))
		test.True(t, q.Push(3))

		runErr := make(chan error, 1)
		go func() { runErr <- q.RunUntilClosed(t.Context()) }()
		synctest.Wait()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		test.ErrorIs(t, q.Drain(ctx), context.DeadlineExceeded)
		test.NoError(t, <-runErr)
		test.Len(t, 0, processed)
		test.Equal(t, 1, len(q.queue.queue))
		test.Equal(t, 1, q.Overflowed())

		close(gate)
		test.NoError(t, q.Run(t.Context()))
		test.Equal(t, []int{2, 3}, processed)
	})
}

func TestOverflowQueueWorkerFailure(t *testing.T) {
	t.Parallel()

	errMock := errors.New("mock")
	var processed []int
	q := NewOverflowQueue(OverflowQueueOptions[int]{
		Size: 1,
		Worker: func(_ context.Context, item int) error {
			if item == 1 {
				return errMock
			}
			processed = append(processed, item)
			return nil
		},
	})
	test.True(t, q.Push(1))
	test.True(t, q.Push(2))
	test.True(t, q.Push(3))

	test.ErrorIs(t, q.Run(t.Context()), errMock)
	test.Equal(t, 1, q.Overflowed())

	q.Close()
	test.NoError(t, q.Run(t.Context()))
	test.Equal(t, []int{2, 3}, processed)
}

func TestOverflowQueueWorkerRequired(t *testing.T) {
	t.Parallel()

	q := NewOverflowQueue(OverflowQueueOptions[int]{})
	test.ErrorIs(t, q.Run(t.Context()), ErrQueueWorkerRequired)
}

func TestOverflowQueueConcurrent(t *testing.T) {
	t.Parallel()

	const (
		items     = 1000
		producers = 8
	)

	tests := []struct {
		bounded bool
		name    string
		order   OverflowOrder
	}{
		{false, "fifo/unlimited", OverflowFIFO},
		{true, "fifo/bounded", OverflowFIFO},
		{false, "lifo/unlimited", OverflowLIFO},
		{true, "lifo/bounded", OverflowLIFO},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				var (
					failed atomic.Int32
					mu     sync.Mutex
					seen   = make(map[int]int)
				)
				opts := OverflowQueueOptions[int]{
					Size:    4,
					Workers: 4,
					Worker: func(_ context.Context, item int) error {
						mu.Lock()
						seen[item]++
						mu.Unlock()
						return nil
					},
				}
				opts.OverflowOrder = tt.order
				if tt.bounded {
					opts.OverflowSize = 8
				}
				q := NewOverflowQueue(opts)

				runErr := make(chan error, 1)
				go func() { runErr <- q.Run(t.Context()) }()

				var wg sync.WaitGroup
				for p := range producers {
					wg.Go(func() {
						for i := range items {
							item := p*items + i
							if tt.bounded {
								if q.PushWait(t.Context(), item) != nil {
									failed.Add(1)
								}
							} else if !q.Push(item) {
								failed.Add(1)
							}
						}
					})
				}
				wg.Wait()

				q.Close()
				test.NoError(t, <-runErr)
				test.Equal(t, int32(0), failed.Load())
				test.Equal(t, producers*items, len(seen))
				for item, n := range seen {
					test.Equal(t, 1, n, "item processed more than once: "+strconv.Itoa(item))
				}
			})
		})
	}
}
