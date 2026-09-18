package worker

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// recordSleeps replaces sleep for one test and returns the waits Retry asked
// for, in order. Nothing actually waits.
func recordSleeps(t *testing.T) *[]time.Duration {
	t.Helper()
	var waits []time.Duration
	orig := sleep
	sleep = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return ctx.Err()
	}
	t.Cleanup(func() { sleep = orig })
	return &waits
}

// failFirst returns an op that fails n times, then succeeds, and a pointer
// to how many times it was called.
func failFirst(n int) (func(context.Context) error, *int) {
	calls := 0
	return func(context.Context) error {
		calls++
		if calls <= n {
			return errors.New("downstream unavailable")
		}
		return nil
	}, &calls
}

func TestRetry_WaitsLongerBeforeEachRetry(t *testing.T) {
	waits := recordSleeps(t)
	op, calls := failFirst(3)
	p := Policy{MaxAttempts: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: 10 * time.Second}

	if err := Retry(context.Background(), p, op); err != nil {
		t.Fatalf("Retry = %v, want success on the fourth attempt", err)
	}
	if *calls != 4 {
		t.Fatalf("op called %d times, want 4", *calls)
	}
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	if !reflect.DeepEqual(*waits, want) {
		t.Errorf("waited %v between attempts, want %v", *waits, want)
	}
}

func TestRetry_NeverWaitsLongerThanMaxDelay(t *testing.T) {
	waits := recordSleeps(t)
	op, _ := failFirst(100)
	p := Policy{MaxAttempts: 6, BaseDelay: time.Second, MaxDelay: 3 * time.Second}

	if err := Retry(context.Background(), p, op); err == nil {
		t.Fatal("Retry succeeded, want an error: every attempt fails")
	}
	want := []time.Duration{time.Second, 2 * time.Second, 3 * time.Second, 3 * time.Second, 3 * time.Second}
	if !reflect.DeepEqual(*waits, want) {
		t.Errorf("waited %v between attempts, want %v", *waits, want)
	}
}

func TestRetry_StopsRetryingOnceTheContextIsCancelled(t *testing.T) {
	recordSleeps(t)
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	op := func(context.Context) error {
		calls++
		cancel() // the worker is shutting down while this attempt runs
		return errors.New("downstream unavailable")
	}

	err := Retry(ctx, DefaultPolicy, op)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Retry = %v, want an error wrapping context.Canceled", err)
	}
	if calls != 1 {
		t.Errorf("op called %d times after shutdown began, want 1", calls)
	}
}
