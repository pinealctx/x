package retryx

import (
	"math"
	"math/rand/v2"
	"time"
)

// BackoffStrategy computes the wait duration before the next retry attempt.
type BackoffStrategy interface {
	Wait(attempt int) time.Duration
}

// Compile-time interface compliance checks.
var (
	_ BackoffStrategy = (*exponentialBackoff)(nil)
	_ BackoffStrategy = (*fixedBackoff)(nil)
	_ BackoffStrategy = (*jitterBackoff)(nil)
	_ BackoffStrategy = (*maxWaitBackoff)(nil)
)

// exponentialBackoff computes base * factor^attempt.
type exponentialBackoff struct {
	base   time.Duration
	factor float64
}

// NewExponential creates an exponential backoff: base * factor^attempt.
// Panics if base <= 0 or factor < 1.
func NewExponential(base time.Duration, factor float64) BackoffStrategy {
	if base <= 0 {
		panic("retryx: NewExponential: base must be > 0")
	}
	if factor < 1 {
		panic("retryx: NewExponential: factor must be >= 1")
	}
	return &exponentialBackoff{base: base, factor: factor}
}

func (e *exponentialBackoff) Wait(attempt int) time.Duration {
	mult := math.Pow(e.factor, float64(attempt))
	d := float64(e.base) * mult
	if d > float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(d)
}

// fixedBackoff returns a constant interval.
type fixedBackoff struct {
	interval time.Duration
}

// NewFixed creates a fixed-interval backoff.
// Panics if interval <= 0.
func NewFixed(interval time.Duration) BackoffStrategy {
	if interval <= 0 {
		panic("retryx: NewFixed: interval must be > 0")
	}
	return &fixedBackoff{interval: interval}
}

func (f *fixedBackoff) Wait(_ int) time.Duration {
	return f.interval
}

// jitterBackoff wraps a strategy with random jitter.
type jitterBackoff struct {
	inner BackoffStrategy
	ratio float64
}

// WithJitter wraps a strategy with random jitter.
// The actual wait is in [wait*(1-ratio), wait*(1+ratio)].
// Panics if ratio <= 0 or ratio >= 1.
func WithJitter(b BackoffStrategy, ratio float64) BackoffStrategy {
	if ratio <= 0 || ratio >= 1 {
		panic("retryx: WithJitter: ratio must be in (0, 1)")
	}
	return &jitterBackoff{inner: b, ratio: ratio}
}

func (j *jitterBackoff) Wait(attempt int) time.Duration {
	w := j.inner.Wait(attempt)
	lo := float64(w) * (1 - j.ratio)
	hi := float64(w) * (1 + j.ratio)
	// Intentionally uses the global rand from math/rand/v2 (ChaCha8 PRNG, thread-safe).
	// Jitter exists to introduce non-determinism; deterministic replay contradicts its
	// purpose. For deterministic backoff in tests, use NewFixed without WithJitter.
	return time.Duration(lo + rand.Float64()*(hi-lo)) //nolint:gosec // G404: intentional non-crypto RNG
}

// maxWaitBackoff caps the actual wait to max.
type maxWaitBackoff struct {
	inner BackoffStrategy
	max   time.Duration
}

// WithMaxWait wraps a strategy to cap the maximum wait duration.
// Panics if mx <= 0.
func WithMaxWait(b BackoffStrategy, mx time.Duration) BackoffStrategy {
	if mx <= 0 {
		panic("retryx: WithMaxWait: max must be > 0")
	}
	return &maxWaitBackoff{inner: b, max: mx}
}

func (m *maxWaitBackoff) Wait(attempt int) time.Duration {
	w := m.inner.Wait(attempt)
	if w > m.max {
		return m.max
	}
	return w
}
