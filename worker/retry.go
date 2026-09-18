// Package worker holds the pieces the sandbox worker is built from.
package worker

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Policy says how a failed job is retried.
type Policy struct {
	// MaxAttempts is the total number of attempts, including the first.
	MaxAttempts int
	// BaseDelay is how long to wait before the first retry. Each retry after
	// that should wait twice as long as the one before it.
	BaseDelay time.Duration
	// MaxDelay caps any single wait, however many retries have gone by.
	MaxDelay time.Duration
}

// DefaultPolicy rides out a downstream that is unavailable for a second or
// two, which is the common case when it is restarting.
var DefaultPolicy = Policy{
	MaxAttempts: 5,
	BaseDelay:   100 * time.Millisecond,
	MaxDelay:    2 * time.Second,
}

// sleep waits for d, or returns early with ctx's error if ctx is done first.
// It is a variable so tests can see how long the worker waited without
// actually waiting.
var sleep = func(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Retry runs op until it succeeds, the policy's attempts run out, or ctx is
// cancelled, and returns the last error if it never succeeds.
func Retry(ctx context.Context, p Policy, op func(ctx context.Context) error) error {
	var err error
	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		if err = op(ctx); err == nil {
			return nil
		}
		log.Printf("attempt %d/%d failed: %v", attempt, p.MaxAttempts, err)
	}
	return fmt.Errorf("giving up after %d attempts: %w", p.MaxAttempts, err)
}
