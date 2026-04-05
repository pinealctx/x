package retryx

import (
	"context"
	"fmt"
	"time"
)

// Option configures retry behavior.
type Option func(*config)

type config struct {
	attempts int
	backoff  BackoffStrategy
	retryIf  func(error) bool
	onRetry  func(attempt int, err error)
}

// Attempts sets the maximum number of attempts (including the first execution, default 3).
// Panics if n <= 0.
func Attempts(n int) Option {
	if n <= 0 {
		panic("retryx: Attempts requires n > 0")
	}
	return func(c *config) {
		c.attempts = n
	}
}

// Backoff sets the backoff strategy (default: no wait, immediate retry).
func Backoff(b BackoffStrategy) Option {
	return func(c *config) {
		c.backoff = b
	}
}

// RetryIf sets the error predicate for retryable errors (default: retry all errors).
func RetryIf(fn func(error) bool) Option {
	return func(c *config) {
		c.retryIf = fn
	}
}

// OnRetry sets a callback invoked before each retry attempt (for logging/metrics).
func OnRetry(fn func(attempt int, err error)) Option {
	return func(c *config) {
		c.onRetry = fn
	}
}

// Do executes fn with retry on failure.
// Returns the result of the first successful execution, or the last error.
// The context is checked before each attempt; if canceled, Do returns ctx.Err().
func Do[T any](ctx context.Context, fn func() (T, error), opts ...Option) (T, error) {
	cfg := config{
		attempts: 3,
	}
	for _, o := range opts {
		o(&cfg)
	}

	var zero T
	var lastErr error
	for attempt := range cfg.attempts {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		val, err := fn()
		if err == nil {
			return val, nil
		}
		lastErr = err

		// No more attempts left.
		if attempt == cfg.attempts-1 {
			break
		}

		// Check if error is retryable.
		if cfg.retryIf != nil && !cfg.retryIf(err) {
			return zero, err
		}

		// Notify callback.
		if cfg.onRetry != nil {
			cfg.onRetry(attempt, err)
		}

		// Wait before next attempt.
		if cfg.backoff != nil {
			wait := cfg.backoff.Wait(attempt)
			if wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					timer.Stop()
					return zero, ctx.Err()
				case <-timer.C:
				}
			}
		}
	}

	return zero, fmt.Errorf("retryx: all %d attempts failed: %w", cfg.attempts, lastErr)
}
