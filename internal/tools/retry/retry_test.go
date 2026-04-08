package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

var errTransient = errors.New("transient")

func TestDoRetriesTransientErrorAndReturnsResult(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	attempts := 0
	var observedDelays []time.Duration

	result, err := Do[int](ctx, Config[int]{
		Attempts:     3,
		InitialDelay: 10 * time.Millisecond,
		MaximumDelay: 25 * time.Millisecond,
		Retryable: func(err error) bool {
			return errors.Is(err, errTransient)
		},
		Wait: func(context.Context, time.Duration) error {
			return nil
		},
		OnRetry: func(attempt int, nextDelay time.Duration, err error) {
			observedDelays = append(observedDelays, nextDelay)
			if attempt != len(observedDelays) {
				t.Fatalf("unexpected attempt number %d", attempt)
			}
			if !errors.Is(err, errTransient) {
				t.Fatalf("unexpected retry error: %v", err)
			}
		},
	}, func(context.Context) (int, error) {
		attempts++
		if attempts < 3 {
			return 0, errTransient
		}

		return 42, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Fatalf("unexpected result: %d", result)
	}
	if attempts != 3 {
		t.Fatalf("unexpected attempt count: %d", attempts)
	}
	if len(observedDelays) != 2 {
		t.Fatalf("unexpected retry count: %d", len(observedDelays))
	}
	if observedDelays[0] != 10*time.Millisecond || observedDelays[1] != 20*time.Millisecond {
		t.Fatalf("unexpected delays: %v", observedDelays)
	}
}

func TestDoStopsOnPermanentError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	attempts := 0
	permanentErr := errors.New("permanent")

	_, err := Do[string](ctx, Config[string]{
		Attempts: 4,
		Retryable: func(error) bool {
			return false
		},
		Wait: func(context.Context, time.Duration) error {
			t.Fatal("wait should not be called")
			return nil
		},
	}, func(context.Context) (string, error) {
		attempts++
		return "", permanentErr
	})

	if !errors.Is(err, permanentErr) {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("unexpected attempt count: %d", attempts)
	}
}

func TestDoReturnsContextErrorFromWait(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	_, err := Do[string](ctx, Config[string]{
		Attempts:     2,
		InitialDelay: time.Millisecond,
		Retryable: func(error) bool {
			return true
		},
		Wait: Wait,
	}, func(context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", fmt.Errorf("retry me")
		}

		return "", nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("unexpected attempt count: %d", attempts)
	}
}

func TestDelayCapsAtMaximum(t *testing.T) {
	t.Parallel()

	if got := Delay(10*time.Millisecond, 25*time.Millisecond, 1); got != 10*time.Millisecond {
		t.Fatalf("unexpected first delay: %s", got)
	}
	if got := Delay(10*time.Millisecond, 25*time.Millisecond, 2); got != 20*time.Millisecond {
		t.Fatalf("unexpected second delay: %s", got)
	}
	if got := Delay(10*time.Millisecond, 25*time.Millisecond, 4); got != 25*time.Millisecond {
		t.Fatalf("unexpected capped delay: %s", got)
	}
}
