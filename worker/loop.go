package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// stableConnectionThreshold is how long a streaming connection must
// remain healthy before RetryWithBackoff resets the reconnect backoff.
// Shared by all stream-based workers so they behave consistently.
const stableConnectionThreshold = 30 * time.Second

// RetryWithBackoff invokes fn repeatedly until ctx is cancelled.
//
// On each non-nil error from fn the loop waits for `delay` before retrying;
// `delay` doubles on every consecutive failure, capped at `maxDelay`, and
// resets back to `baseDelay` whenever fn ran for at least `resetAfter`
// before returning (i.e. the connection or operation was stable for a
// while before failing). context.Canceled errors from fn are treated as a
// signal to exit, not retry.
func RetryWithBackoff(
	ctx context.Context,
	name string,
	baseDelay, maxDelay, resetAfter time.Duration,
	fn func(context.Context) error,
) {
	delay := baseDelay

	for {
		if ctx.Err() != nil {
			return
		}

		startedAt := time.Now()
		err := fn(ctx)

		if ctx.Err() != nil {
			return
		}

		if errors.Is(err, context.Canceled) {
			return
		}

		if time.Since(startedAt) >= resetAfter {
			delay = baseDelay
		}

		// A clean return is treated as "ran to completion, immediately
		// re-enter" — no backoff and no warning. Backoff only on errors.
		if err == nil {
			continue
		}

		slog.Warn("worker iteration failed; retrying after backoff",
			"name", name, "delay", delay, "error", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		delay = min(delay*2, maxDelay) //nolint:mnd // exponential backoff factor
	}
}
