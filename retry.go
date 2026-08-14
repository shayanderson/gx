package gx

import (
	"context"
	"errors"
	"time"
)

// RetryFunc is the function executed by Retry.
type RetryFunc func(context.Context) error

// RetryOptions configures a Retry.
type RetryOptions struct {
	// Attempts is the total number of attempts, including the first.
	// A value of zero disables the attempt limit.
	Attempts int

	// Backoff is the exponential multiplier applied to the delay
	// after each failed attempt.
	// A value of zero disables backoff, using a fixed delay.
	Backoff float64

	// Delay is the delay between failed attempts.
	Delay time.Duration

	// MaxDelay limits the maximum delay between attempts.
	// A value of zero disables the limit.
	MaxDelay time.Duration

	// MaxDuration limits the total duration of all attempts by applying
	// a timeout to the retry context.
	// A value of zero disables the limit.
	MaxDuration time.Duration
}

// Retry retries a function using configurable limits, delay, and backoff.
type Retry struct {
	attempts    int
	backoff     float64
	delay       time.Duration
	maxDelay    time.Duration
	maxDuration time.Duration
}

// NewRetry returns a new Retry configured with opts.
func NewRetry(opts RetryOptions) (*Retry, error) {
	if opts.Attempts < 0 {
		return nil, errors.New("attempts must not be negative")
	}
	if opts.Delay < 0 {
		return nil, errors.New("delay must not be negative")
	}
	if opts.Backoff < 0 {
		return nil, errors.New("backoff must not be negative")
	}
	if opts.MaxDelay < 0 {
		return nil, errors.New("max delay must not be negative")
	}
	if opts.MaxDuration < 0 {
		return nil, errors.New("max duration must not be negative")
	}
	if opts.Attempts == 0 && opts.MaxDuration == 0 && opts.Delay == 0 {
		return nil, errors.New("delay is required for unbounded retries")
	}

	backoff := opts.Backoff
	if backoff == 0 {
		backoff = 1
	}

	return &Retry{
		attempts:    opts.Attempts,
		delay:       opts.Delay,
		backoff:     backoff,
		maxDelay:    opts.MaxDelay,
		maxDuration: opts.MaxDuration,
	}, nil
}

// Do executes fn until it succeeds, the maximum number of attempts is reached,
// the retry context is done, or MaxDuration is exceeded. If Attempts and
// MaxDuration are both zero, retries continue until fn succeeds or ctx is done.
// It returns nil on success, the retry context error when canceled or
// timed out, or the last error returned by fn when the attempt limit is reached.
func (r *Retry) Do(ctx context.Context, fn RetryFunc) error {
	if r.maxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.maxDuration)
		defer cancel()
	}

	delay := r.delay
	var err error

	for attempt := 0; ; attempt++ {
		err = fn(ctx)
		if err == nil {
			return nil
		}

		if r.attempts > 0 && attempt == r.attempts-1 {
			break
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if delay > 0 {
			timer := time.NewTimer(delay)

			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return ctx.Err()

			case <-timer.C:
			}
		}

		delay = time.Duration(float64(delay) * r.backoff)

		if r.maxDelay > 0 && delay > r.maxDelay {
			delay = r.maxDelay
		}
	}

	return err
}
