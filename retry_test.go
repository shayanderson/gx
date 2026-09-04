package gx

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestNewRetry(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{
		Attempts:    3,
		Backoff:     2,
		Delay:       time.Millisecond,
		MaxDelay:    time.Second,
		MaxDuration: time.Minute,
	})

	test.NoError(t, err)
	test.Equal(t, 3, r.attempts)
	test.Equal(t, 2.0, r.backoff)
	test.Equal(t, time.Millisecond, r.delay)
	test.Equal(t, time.Second, r.maxDelay)
	test.Equal(t, time.Minute, r.maxDuration)
}

func TestNewRetryInvalidOptions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts RetryOptions
	}{
		{name: "attempts", opts: RetryOptions{Attempts: -1, Backoff: 1}},
		{name: "delay", opts: RetryOptions{Attempts: 1, Backoff: 1, Delay: -time.Millisecond}},
		{name: "backoff", opts: RetryOptions{Attempts: 1, Backoff: -1}},
		{
			name: "max_delay",
			opts: RetryOptions{Attempts: 1, Backoff: 1, MaxDelay: -time.Millisecond},
		},
		{
			name: "max_duration",
			opts: RetryOptions{Attempts: 1, Backoff: 1, MaxDuration: -time.Millisecond},
		},
		{name: "unbounded_zero_delay", opts: RetryOptions{Backoff: 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewRetry(tc.opts)

			test.Nil(t, r)
			test.NotNil(t, err)
		})
	}
}

func TestRetryDoSuccess(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{Attempts: 3})
	test.NoError(t, err)
	calls := 0

	err = r.Do(t.Context(), func(context.Context) error {
		calls++
		return nil
	})

	test.NoError(t, err)
	test.Equal(t, 1, calls)
}

func TestNewRetryDefaultBackoff(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{Attempts: 3})

	test.NoError(t, err)
	test.Equal(t, 1.0, r.backoff)
}

func TestRetryDoEventuallySucceeds(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{Attempts: 3})
	test.NoError(t, err)
	failErr := errors.New("failed")
	calls := 0

	err = r.Do(t.Context(), func(context.Context) error {
		calls++
		if calls < 3 {
			return failErr
		}
		return nil
	})

	test.NoError(t, err)
	test.Equal(t, 3, calls)
}

func TestRetryDoReturnsLastError(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{Attempts: 3})
	test.NoError(t, err)
	firstErr := errors.New("first")
	lastErr := errors.New("last")
	calls := 0

	err = r.Do(t.Context(), func(context.Context) error {
		calls++
		if calls == 1 {
			return firstErr
		}
		return lastErr
	})

	test.Error(t, err, lastErr)
	test.Equal(t, 3, calls)
}

func TestRetryDoContextCanceled(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{Attempts: 3, Delay: time.Second})
	test.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	calls := 0

	err = r.Do(ctx, func(context.Context) error {
		calls++
		cancel()
		return errors.New("failed")
	})

	test.Error(t, err, context.Canceled)
	test.Equal(t, 1, calls)
}

func TestRetryDoZeroDelay(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{Attempts: 3, Backoff: 2})
	test.NoError(t, err)
	failErr := errors.New("failed")
	calls := 0

	err = r.Do(t.Context(), func(context.Context) error {
		calls++
		return failErr
	})

	test.Error(t, err, failErr)
	test.Equal(t, 3, calls)
}

func TestRetryDoMaxDelay(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{
		Attempts: 3,
		Backoff:  10,
		Delay:    time.Millisecond,
		MaxDelay: time.Millisecond,
	})
	test.NoError(t, err)
	failErr := errors.New("failed")
	calls := 0
	start := time.Now()

	err = r.Do(t.Context(), func(context.Context) error {
		calls++
		return failErr
	})

	test.Error(t, err, failErr)
	test.Equal(t, 3, calls)
	test.GreaterOrEqual(t, time.Since(start), 2*time.Millisecond)
}

func TestRetryDoBackoffIncreasesDelay(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{
		Attempts: 4,
		Backoff:  2,
		Delay:    5 * time.Millisecond,
	})
	test.NoError(t, err)
	failErr := errors.New("failed")
	calls := 0
	start := time.Now()

	err = r.Do(t.Context(), func(context.Context) error {
		calls++
		return failErr
	})

	test.Error(t, err, failErr)
	test.Equal(t, 4, calls)
	test.GreaterOrEqual(t, time.Since(start), 35*time.Millisecond)
}

func TestRetryDoAttemptsWinBeforeMaxDuration(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{
		Attempts:    2,
		Delay:       time.Millisecond,
		MaxDuration: time.Second,
	})
	test.NoError(t, err)
	failErr := errors.New("failed")
	calls := 0

	err = r.Do(t.Context(), func(context.Context) error {
		calls++
		return failErr
	})

	test.Error(t, err, failErr)
	test.Equal(t, 2, calls)
}

func TestRetryDoMaxDuration(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{
		Attempts:    10,
		Delay:       20 * time.Millisecond,
		MaxDuration: 25 * time.Millisecond,
	})
	test.NoError(t, err)
	failErr := errors.New("failed")
	calls := 0

	err = r.Do(t.Context(), func(context.Context) error {
		calls++
		return failErr
	})

	test.Error(t, err, context.DeadlineExceeded)
	test.True(t, calls >= 1)
	test.True(t, calls < 10)
}

func TestRetryDoMaxDurationOnly(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{
		Delay:       10 * time.Millisecond,
		MaxDuration: 25 * time.Millisecond,
	})
	test.NoError(t, err)
	failErr := errors.New("failed")
	calls := 0

	err = r.Do(t.Context(), func(context.Context) error {
		calls++
		return failErr
	})

	test.Error(t, err, context.DeadlineExceeded)
	test.True(t, calls >= 1)
}

func TestRetryDoMaxDurationStopsAfterSlowAttempt(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{
		Attempts:    10,
		MaxDuration: 10 * time.Millisecond,
	})
	test.NoError(t, err)
	failErr := errors.New("failed")
	calls := 0

	err = r.Do(t.Context(), func(context.Context) error {
		calls++
		time.Sleep(15 * time.Millisecond)
		return failErr
	})

	test.Error(t, err, context.DeadlineExceeded)
	test.Equal(t, 1, calls)
}

func TestRetryDoMaxDurationPassesTimeoutContext(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{Attempts: 3, MaxDuration: time.Second})
	test.NoError(t, err)

	err = r.Do(t.Context(), func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		test.True(t, ok)
		return nil
	})

	test.NoError(t, err)
}

func TestRetryDoUnboundedUntilContextCanceled(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{Delay: time.Millisecond})
	test.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	calls := 0

	err = r.Do(ctx, func(context.Context) error {
		calls++
		if calls == 3 {
			cancel()
		}
		return errors.New("failed")
	})

	test.Error(t, err, context.Canceled)
	test.Equal(t, 3, calls)
}

func TestRetryDoParentContextDeadlineWins(t *testing.T) {
	t.Parallel()

	r, err := NewRetry(RetryOptions{
		MaxDuration: time.Second,
	})
	test.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Millisecond)
	defer cancel()

	err = r.Do(ctx, func(context.Context) error {
		return errors.New("failed")
	})

	test.Error(t, err, context.DeadlineExceeded)
}
