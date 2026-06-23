package main

import (
	"context"
	"fmt"
	"time"

	"billing/base"

	log "github.com/sirupsen/logrus"
)

// retryPolicy controls retryDBConnect's timing.
type retryPolicy struct {
	budget  time.Duration // total time to keep trying before giving up
	initial time.Duration // first backoff
	cap     time.Duration // max backoff between attempts
}

// migratePolicy is the boot-time policy: keep retrying the migration for ~60s
// with exponential backoff capped at 5s, so billing waits for a postgres that
// is still starting instead of crash-looping.
var migratePolicy = retryPolicy{budget: 60 * time.Second, initial: 500 * time.Millisecond, cap: 5 * time.Second}

// retryDBConnect calls fn until it succeeds, the budget is exhausted, or fn
// returns a non-retryable error. Transient connect errors are logged at warn,
// configuration rejections (bad password / missing DB) at error so a real
// misconfiguration is never silently retried. A non-connection error (e.g. a
// broken migration) is returned immediately so it fails fast.
func retryDBConnect(ctx context.Context, op string, p retryPolicy, fn func(context.Context) error) error {
	deadline := time.Now().Add(p.budget)
	backoff := p.initial
	for attempt := 1; ; attempt++ {
		err := fn(ctx)
		if err == nil {
			if attempt > 1 {
				log.Infof("%s: connected after %d attempts", op, attempt)
			}
			return nil
		}
		if !base.IsRetryableConnErr(err) {
			return err // real error — do not retry
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s: giving up after %s and %d attempts: %w", op, p.budget, attempt, err)
		}
		if base.IsConfigConnErr(err) {
			log.Errorf("%s: database misconfigured, retrying in %s (attempt %d): %v", op, backoff, attempt, err)
		} else {
			log.Warnf("%s: postgres unreachable, retrying in %s (attempt %d): %v", op, backoff, attempt, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > p.cap {
			backoff = p.cap
		}
	}
}
