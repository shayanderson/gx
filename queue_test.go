package gx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
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
	test.NoError(t, q.Run(t.Context()))
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
