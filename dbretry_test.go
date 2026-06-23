package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var transient = errors.New("connection refused") // retryable connect failure

func fastPolicy() retryPolicy {
	return retryPolicy{budget: 5 * time.Second, initial: time.Millisecond, cap: time.Millisecond}
}

func TestRetrySucceedsAfterTransient(t *testing.T) {
	attempts := 0
	err := retryDBConnect(context.Background(), "test", fastPolicy(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return transient
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryFailsFastOnNonConnError(t *testing.T) {
	boom := errors.New("dirty migration version 7")
	attempts := 0
	err := retryDBConnect(context.Background(), "test", fastPolicy(), func(context.Context) error {
		attempts++
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom returned unwrapped, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("non-retryable error must not retry; got %d attempts", attempts)
	}
}

func TestRetryExhaustsBudget(t *testing.T) {
	p := retryPolicy{budget: 20 * time.Millisecond, initial: 2 * time.Millisecond, cap: 5 * time.Millisecond}
	err := retryDBConnect(context.Background(), "test", p, func(context.Context) error {
		return transient
	})
	if err == nil {
		t.Fatal("expected error after budget exhausted")
	}
	if !strings.Contains(err.Error(), "giving up") {
		t.Fatalf("expected 'giving up' error, got %v", err)
	}
	if !errors.Is(err, transient) {
		t.Fatalf("budget error should wrap the last transient error, got %v", err)
	}
}

func TestRetryHonorsContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	p := retryPolicy{budget: 5 * time.Second, initial: 200 * time.Millisecond, cap: 200 * time.Millisecond}
	err := retryDBConnect(ctx, "test", p, func(context.Context) error {
		return transient // always transient, so we only exit via ctx
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline error, got %v", err)
	}
}
