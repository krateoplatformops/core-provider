package retry

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Config[T any] struct {
	Attempts     int
	InitialDelay time.Duration
	MaximumDelay time.Duration
	Wait         func(context.Context, time.Duration) error
	Retryable    func(error) bool
	OnRetry      func(attempt int, nextDelay time.Duration, err error)
}

func Do[T any](ctx context.Context, cfg Config[T], operation func(context.Context) (T, error)) (T, error) {
	var zeroValue T

	attempts := cfg.Attempts
	if attempts <= 0 {
		attempts = 1
	}

	wait := cfg.Wait
	if wait == nil {
		wait = Wait
	}

	retryable := cfg.Retryable
	if retryable == nil {
		retryable = func(error) bool { return false }
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		result, err := operation(ctx)
		if err == nil {
			return result, nil
		}

		if !retryable(err) || attempt == attempts {
			return zeroValue, err
		}

		delay := Delay(cfg.InitialDelay, cfg.MaximumDelay, attempt)
		if cfg.OnRetry != nil {
			cfg.OnRetry(attempt, delay, err)
		}

		if err := wait(ctx, delay); err != nil {
			return zeroValue, err
		}
	}

	return zeroValue, fmt.Errorf("retry exhausted unexpectedly")
}

func Delay(initial, maximum time.Duration, attempt int) time.Duration {
	if attempt <= 1 {
		if maximum > 0 && initial > maximum {
			return maximum
		}
		return initial
	}

	delay := initial << (attempt - 1)
	if maximum > 0 && delay > maximum {
		return maximum
	}

	return delay
}

func Wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func IsContextCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}
