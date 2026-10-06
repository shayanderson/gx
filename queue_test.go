package gx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestQueue(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processed atomic.Int32

	q := NewQueue(QueueOptions[int]{
		Workers: 1,
		Size:    2,
		Worker: func(context.Context, int) error {
			processed.Add(1)
			return nil
		},
	},
	)

	errCh := make(chan error, 1)

	go func() {
		errCh <- q.Run(ctx)
	}()

	test.True(t, q.Push(1))

	test.True(t, q.Push(2))

	for processed.Load() < 2 {
		time.Sleep(time.Millisecond)
	}

	cancel()

	test.NoError(t, <-errCh)

	test.Equal(t, int32(2), processed.Load())
}

func TestQueueDefaults(t *testing.T) {
	t.Parallel()

	q := NewQueue(QueueOptions[int]{
		Workers: 0,
		Size:    0,
		Worker: func(context.Context, int) error {
			return nil
		},
	})

	test.Equal(t, 1, q.workers)
	test.Equal(t, 4, cap(q.queue))
}

func TestQueueFull(t *testing.T) {
	t.Parallel()

	q := NewQueue(QueueOptions[int]{
		Workers: 1,
		Size:    1,
		Worker: func(context.Context, int) error {
			time.Sleep(time.Second)
			return nil
		},
	})

	test.True(t, q.Push(1))

	test.False(t, q.Push(2))
	test.False(t, q.Closed())
}

func TestQueueFailOnFull(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	var once sync.Once

	q := NewQueue(QueueOptions[int]{
		FailOnFull: true,
		Size:       1,
		Worker: func(ctx context.Context, _ int) error {
			once.Do(func() { close(started) })
			<-ctx.Done()
			return nil
		},
	})

	errs := make(chan error, 1)
	go func() { errs <- q.Run(t.Context()) }()

	test.True(t, q.Push(1))
	<-started
	test.True(t, q.Push(2))
	test.False(t, q.Push(3))
	test.True(t, errors.Is(<-errs, ErrQueueFull))
}

func TestQueueFailOnFullBeforeRun(t *testing.T) {
	t.Parallel()

	q := NewQueue(QueueOptions[int]{
		FailOnFull: true,
		Size:       1,
		Worker:     func(context.Context, int) error { return nil },
	})

	test.True(t, q.Push(1))
	test.False(t, q.Push(2))

	q.Close()
	test.True(t, errors.Is(q.Run(t.Context()), ErrQueueFull))
}

func TestQueueFailOnFullDoesNotFailAfterClose(t *testing.T) {
	t.Parallel()

	q := NewQueue(QueueOptions[int]{
		FailOnFull: true,
		Size:       1,
		Worker:     func(context.Context, int) error { return nil },
	})

	q.Close()

	test.False(t, q.Push(1))
	test.NoError(t, q.Run(t.Context()))
}

func TestQueueFailNil(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var processed int
		q := NewQueue(QueueOptions[int]{
			Worker: func(context.Context, int) error {
				processed++
				return nil
			},
		})
		errs := make(chan error, 1)
		go func() { errs <- q.Run(t.Context()) }()
		synctest.Wait()

		q.fail(nil)
		synctest.Wait()
		test.Equal(t, 0, len(errs)) // A nil error must not cancel Run.
		test.True(t, q.Push(1))
		q.Close()
		test.NoError(t, <-errs)
		test.Equal(t, 1, processed)
	})
}

func TestQueueFailPreservesFirstError(t *testing.T) {
	t.Parallel()

	q := NewQueue(QueueOptions[int]{
		Worker: func(context.Context, int) error { return nil },
	})
	first := errors.New("first failure")
	second := errors.New("second failure")

	q.fail(first)
	q.fail(second)
	q.fail(nil)

	// Close and Run also verify that the repeated-failure path releases mu.
	q.Close()
	test.True(t, errors.Is(q.Run(t.Context()), first))
}

func TestQueueWorkerError(t *testing.T) {
	t.Parallel()

	expected := errors.New("test error")

	q := NewQueue(QueueOptions[int]{
		Workers: 1,
		Size:    2,
		Worker: func(context.Context, int) error {
			return expected
		},
	})
	defer q.Close()

	errs := make(chan error, 1)

	go func() {
		errs <- q.Run(t.Context())
	}()

	time.Sleep(time.Millisecond)

	test.True(t, q.Push(1))

	err := <-errs

	test.NotNil(t, err)
	test.True(t, errors.Is(err, expected))
}

func TestQueueNilWorker(t *testing.T) {
	t.Parallel()

	q := NewQueue(QueueOptions[int]{
		Workers: 1,
		Size:    1,
	})

	err := q.Run(t.Context())

	test.NotNil(t, err)
	test.True(t, errors.Is(err, ErrQueueWorkerRequired))
}

func TestQueueClose(t *testing.T) {
	t.Parallel()

	q := NewQueue(QueueOptions[int]{
		Workers: 1,
		Size:    1,
		Worker: func(context.Context, int) error {
			return nil
		},
	})

	test.False(t, q.Closed())

	q.Close()
	test.True(t, q.Closed())

	err := q.Run(t.Context())

	test.NoError(t, err)

	q.Close() // should not panic
}

func TestQueueContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	q := NewQueue(QueueOptions[int]{
		Workers: 1,
		Size:    1,
		Worker: func(context.Context, int) error {
			return nil
		},
	})

	errCh := make(chan error, 1)

	go func() {
		errCh <- q.Run(ctx)
	}()

	cancel()

	test.NoError(t, <-errCh)
}

func TestQueueContextCancelCause(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)

	q := NewQueue(QueueOptions[int]{
		Worker: func(context.Context, int) error {
			return nil
		},
	})

	errs := make(chan error, 1)
	go func() {
		errs <- q.Run(ctx)
	}()

	cancel(errors.New("parent failed"))
	test.NoError(t, <-errs)
}

func TestQueueDeadlineExceeded(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Millisecond,
	)
	defer cancel()

	q := NewQueue(QueueOptions[int]{
		Workers: 1,
		Size:    1,
		Worker: func(context.Context, int) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	})

	err := q.Run(ctx)

	test.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestQueuePushAfterClose(t *testing.T) {
	t.Parallel()

	q := NewQueue(QueueOptions[int]{
		Workers: 1,
		Size:    10,
		Worker: func(ctx context.Context, i int) error {
			return nil
		},
	})

	done := make(chan any, 1)

	go func() {
		defer func() {
			done <- recover()
		}()

		for {
			if !q.Push(1) {
				return
			}
		}
	}()

	time.Sleep(time.Millisecond)
	q.Close()

	test.Nil(t, <-done)
}

func TestQueuePushClosed(t *testing.T) {
	t.Parallel()

	q := NewQueue(QueueOptions[int]{
		Workers: 1,
		Size:    1,
		Worker: func(ctx context.Context, i int) error {
			return nil
		},
	})

	q.Close()

	test.False(t, q.Push(1))
	test.True(t, errors.Is(q.PushWait(t.Context(), 1), ErrQueueClosed))
	test.Equal(t, int64(0), q.pushWaiters.Load())
	test.True(t, q.Closed())
}

func TestQueueAlreadyRunning(t *testing.T) {
	t.Parallel()

	q := NewQueue(QueueOptions[int]{
		Workers: 1,
		Size:    1,
		Worker: func(ctx context.Context, i int) error {
			return nil
		},
	})

	errCh := make(chan error, 1)

	go func() {
		errCh <- q.Run(t.Context())
	}()

	time.Sleep(time.Millisecond)

	err := q.Run(t.Context())

	test.NotNil(t, err)
	test.True(t, errors.Is(err, ErrQueueAlreadyRunning))

	q.Close()

	test.NoError(t, <-errCh)
}

func TestQueueWorkers(t *testing.T) {
	t.Parallel()

	const workers = 4
	const jobs = 8

	var running atomic.Int32
	var maxRunning atomic.Int32
	started := make(chan struct{}, jobs)

	q := NewQueue(QueueOptions[int]{
		Workers: workers,
		Size:    jobs,
		Worker: func(ctx context.Context, i int) error {
			n := running.Add(1)
			defer running.Add(-1)

			for {
				old := maxRunning.Load()
				if n <= old || maxRunning.CompareAndSwap(old, n) {
					break
				}
			}

			started <- struct{}{}

			time.Sleep(10 * time.Millisecond)

			return nil
		},
	})

	errCh := make(chan error, 1)

	go func() {
		errCh <- q.Run(t.Context())
	}()

	for i := range jobs {
		test.True(t, q.Push(i))
	}

	for range jobs {
		<-started
	}

	q.Close()

	test.NoError(t, <-errCh)

	test.Equal(t, int32(workers), maxRunning.Load())
}

func TestQueuePushWaitBeforeRun(t *testing.T) {
	t.Parallel()

	var processed []int
	q := NewQueue(QueueOptions[int]{
		Size: 2,
		Worker: func(_ context.Context, item int) error {
			processed = append(processed, item)
			return nil
		},
	})

	test.NoError(t, q.PushWait(t.Context(), 1))
	test.True(t, q.Push(2))
	test.Equal(t, 0, len(processed)) // Enqueued, not processed.
	q.Close()
	test.NoError(t, q.Run(t.Context()))
	test.Equal(t, 2, len(processed))
	for i, item := range processed {
		test.Equal(t, i+1, item)
	}
}

func TestQueuePushWaitCapacity(t *testing.T) {
	t.Parallel()

	for _, failOnFull := range []bool{false, true} {
		name := "default"
		if failOnFull {
			name = "fail_on_full"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const producers = 4
				process := make(chan struct{})
				var processed []int
				q := NewQueue(QueueOptions[int]{
					Size:       1,
					FailOnFull: failOnFull,
					Worker: func(_ context.Context, item int) error {
						<-process
						processed = append(processed, item)
						return nil
					},
				})
				test.True(t, q.Push(0))
				pushErrs := make(chan error, producers)
				for i := range producers {
					go func() { pushErrs <- q.PushWait(t.Context(), i+1) }()
				}
				synctest.Wait()
				test.Equal(t, 0, len(pushErrs)) // All producers are blocked.

				runErr := make(chan error, 1)
				go func() { runErr <- q.Run(t.Context()) }()
				for i := range producers {
					synctest.Wait()
					// One enqueue succeeds per dequeue, even with the worker blocked.
					test.Equal(t, 1, len(pushErrs))
					test.NoError(t, <-pushErrs)
					if i < producers-1 {
						process <- struct{}{}
					}
				}

				q.Close()
				close(process)
				test.NoError(t, <-runErr)
				test.Equal(t, producers+1, len(processed))
				seen := make(map[int]bool)
				for _, item := range processed {
					test.False(t, seen[item])
					seen[item] = true
				}
				for i := range producers + 1 {
					test.True(t, seen[i])
				}
			})
		})
	}
}

func TestQueuePushWaitCanceled(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		q := NewQueue(QueueOptions[int]{Size: 1})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		test.True(t, errors.Is(q.PushWait(ctx, 1), context.Canceled))
		test.Equal(t, int64(0), q.pushWaiters.Load())
		test.True(t, q.Push(2)) // The canceled call did not consume capacity.

		ctx, cancel = context.WithCancel(t.Context())
		defer cancel()
		errs := make(chan error, 1)
		go func() { errs <- q.PushWait(ctx, 3) }()
		synctest.Wait()
		test.Equal(t, 0, len(errs))
		cancel()
		test.True(t, errors.Is(<-errs, context.Canceled))
		test.Equal(t, int64(0), q.pushWaiters.Load())
		q.Close()
		test.Equal(t, 2, <-q.queue)
		_, ok := <-q.queue
		test.False(t, ok)
	})
}

func TestQueuePushWaitDeadlineExceeded(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		q := NewQueue(QueueOptions[int]{Size: 1})
		test.True(t, q.Push(1))
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		test.True(t, errors.Is(q.PushWait(ctx, 2), context.DeadlineExceeded))
		test.Equal(t, int64(0), q.pushWaiters.Load())
		q.Close()
		test.Equal(t, 1, <-q.queue)
		_, ok := <-q.queue
		test.False(t, ok)
	})
}

func TestQueuePushWaitClose(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const producers = 8
		var processed []int
		q := NewQueue(QueueOptions[int]{
			Size: 1,
			Worker: func(_ context.Context, item int) error {
				processed = append(processed, item)
				return nil
			},
		})
		test.True(t, q.Push(1))
		errs := make(chan error, producers)
		for range producers {
			go func() { errs <- q.PushWait(t.Context(), 2) }()
		}
		synctest.Wait()
		test.Equal(t, 0, len(errs))
		q.Close()
		q.Close()
		for range producers {
			test.True(t, errors.Is(<-errs, ErrQueueClosed))
		}
		test.Equal(t, int64(0), q.pushWaiters.Load())
		test.NoError(t, q.Run(t.Context()))
		test.Equal(t, 1, len(processed))
		if len(processed) == 1 {
			test.Equal(t, 1, processed[0])
		}
	})
}

func TestQueuePushWaitConcurrent(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const producers, items = 8, 100
		var processed atomic.Int32
		q := NewQueue(QueueOptions[int]{
			Size:    1,
			Workers: 4,
			Worker: func(context.Context, int) error {
				processed.Add(1)
				return nil
			},
		})
		runErr := make(chan error, 1)
		go func() { runErr <- q.Run(t.Context()) }()
		var wg sync.WaitGroup
		for range producers {
			wg.Go(func() {
				for i := range items {
					test.NoError(t, q.PushWait(t.Context(), i))
				}
			})
		}
		// No cancellation or closure can rescue a missed capacity notification.
		wg.Wait()
		test.Equal(t, int64(0), q.pushWaiters.Load())
		q.Close()
		test.NoError(t, <-runErr)
		test.Equal(t, int32(producers*items), processed.Load())
	})
}

func TestQueuePushWaitConcurrentClose(t *testing.T) {
	t.Parallel()

	var accepted, processed atomic.Int32
	started := make(chan struct{})
	var once sync.Once
	q := NewQueue(QueueOptions[int]{
		Size:    2,
		Workers: 4,
		Worker: func(context.Context, int) error {
			processed.Add(1)
			once.Do(func() { close(started) })
			return nil
		},
	})
	runErr := make(chan error, 1)
	go func() { runErr <- q.Run(t.Context()) }()
	var producers sync.WaitGroup
	for range 16 {
		producers.Go(func() {
			for i := range 100 {
				if i%2 == 0 {
					if q.Push(i) {
						accepted.Add(1)
					}
					continue
				}
				err := q.PushWait(t.Context(), i)
				if errors.Is(err, ErrQueueClosed) {
					return
				}
				test.NoError(t, err)
				if err != nil {
					return
				}
				accepted.Add(1)
			}
		})
	}
	<-started
	q.Close()
	producers.Wait()
	test.NoError(t, <-runErr)
	test.Equal(t, accepted.Load(), processed.Load())
	test.True(t, errors.Is(q.PushWait(t.Context(), 0), ErrQueueClosed))
}
