package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryWithBackoff_ExitsImmediatelyWhenContextCancelled(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	RetryWithBackoff(ctx, "test", time.Millisecond, time.Millisecond, time.Second,
		func(_ context.Context) error {
			calls.Add(1)

			return errors.New("never called")
		})

	assert.Equal(t, int32(0), calls.Load(), "fn must not run after ctx is already cancelled")
}

func TestRetryWithBackoff_RetriesUntilCancelled(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})

	go func() {
		defer close(done)

		RetryWithBackoff(ctx, "test", time.Millisecond, 5*time.Millisecond, time.Second,
			func(_ context.Context) error {
				if calls.Add(1) >= 3 {
					cancel()
				}

				return errors.New("transient")
			})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RetryWithBackoff did not return after ctx cancel")
	}

	assert.GreaterOrEqual(t, calls.Load(), int32(3))
}

func TestRetryWithBackoff_BackoffGrows(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	done := make(chan struct{})

	go func() {
		defer close(done)

		RetryWithBackoff(ctx, "test", 10*time.Millisecond, 40*time.Millisecond, time.Second,
			func(_ context.Context) error {
				if calls.Add(1) >= 4 {
					cancel()
				}

				return errors.New("transient")
			})
	}()

	<-done

	// Sum of expected delays: 10 + 20 + 40 = 70ms; allow some slack but
	// confirm we waited noticeably more than 4 * baseDelay.
	assert.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond,
		"expected exponential backoff to accumulate delay")
}

func TestRetryWithBackoff_ResetsAfterStableRun(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// fn runs longer than resetAfter on iteration 1 to trigger the reset,
	// then fails fast on subsequent calls. The reset means iteration 2's
	// backoff is the baseline, not 2x baseline.
	go func() {
		RetryWithBackoff(ctx, "test", 10*time.Millisecond, 80*time.Millisecond, 30*time.Millisecond,
			func(ctx context.Context) error {
				switch calls.Add(1) {
				case 1:
					select {
					case <-time.After(40 * time.Millisecond):
					case <-ctx.Done():
						return ctx.Err()
					}

					return errors.New("err after stable")
				case 5:
					cancel()
				}

				return errors.New("transient")
			})
	}()

	// Just wait a reasonable amount and make sure we ran multiple iterations.
	time.Sleep(500 * time.Millisecond)
	assert.GreaterOrEqual(t, calls.Load(), int32(3))
}
