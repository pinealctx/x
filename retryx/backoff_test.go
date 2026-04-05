// Package retryx is tested with stdlib only (testing package).
// Property-based testing (rapid/gopter) is intentionally not used to keep
// zero external dependencies per the project constraint.
package retryx

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestExponential_Wait(t *testing.T) {
	e := NewExponential(time.Second, 2.0)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
	}
	for _, tt := range tests {
		got := e.Wait(tt.attempt)
		if got != tt.expected {
			t.Errorf("Wait(%d) = %s, want %s", tt.attempt, got, tt.expected)
		}
	}
}

func TestExponential_FactorOne(t *testing.T) {
	e := NewExponential(500*time.Millisecond, 1.0)
	for attempt := range 10 {
		got := e.Wait(attempt)
		if got != 500*time.Millisecond {
			t.Errorf("Wait(%d) = %s, want 500ms with factor=1.0", attempt, got)
		}
	}
}

func TestExponential_NegativeAttempt(t *testing.T) {
	e := NewExponential(time.Second, 2.0)
	// attempt=-1: 1s * 2^(-1) = 500ms
	got := e.Wait(-1)
	if got != 500*time.Millisecond {
		t.Errorf("Wait(-1) = %s, want 500ms", got)
	}
}

func TestExponential_NanosecondBase(t *testing.T) {
	e := NewExponential(1*time.Nanosecond, 2.0)
	got := e.Wait(10)
	// 1ns * 2^10 = 1024ns
	if got != 1024*time.Nanosecond {
		t.Errorf("Wait(10) = %s, want 1024ns", got)
	}
}

func TestExponential_PanicOnZeroBase(t *testing.T) {
	assertPanic(t, "requires base > 0", func() {
		NewExponential(0, 2.0)
	})
}

func TestExponential_PanicOnNegativeBase(t *testing.T) {
	assertPanic(t, "requires base > 0", func() {
		NewExponential(-1*time.Second, 2.0)
	})
}

func TestExponential_PanicOnFactorBelowOne(t *testing.T) {
	assertPanic(t, "requires factor >= 1", func() {
		NewExponential(time.Second, 0.5)
	})
}

func TestFixed_Wait(t *testing.T) {
	f := NewFixed(200 * time.Millisecond)
	for attempt := range 10 {
		got := f.Wait(attempt)
		if got != 200*time.Millisecond {
			t.Errorf("Wait(%d) = %s, want 200ms", attempt, got)
		}
	}
}

func TestFixed_PanicOnZeroInterval(t *testing.T) {
	assertPanic(t, "requires interval > 0", func() {
		NewFixed(0)
	})
}

func TestFixed_PanicOnNegativeInterval(t *testing.T) {
	assertPanic(t, "requires interval > 0", func() {
		NewFixed(-1 * time.Second)
	})
}

func TestJitter_WithinRange(t *testing.T) {
	// Tests range bounds only, not distribution uniformity.
	// For jitter, the critical invariant is "output stays within [lo, hi]";
	// distribution shape is irrelevant to correctness.
	const ratio = 0.1
	base := 1 * time.Second
	inner := NewFixed(base)
	j := WithJitter(inner, ratio)

	lo := time.Duration(float64(base) * (1 - ratio))
	hi := time.Duration(float64(base) * (1 + ratio))

	for i := range 1000 {
		got := j.Wait(0)
		if got < lo || got > hi {
			t.Errorf("sample %d: Wait(0) = %s, want [%s, %s]", i, got, lo, hi)
		}
	}
}

func TestJitter_PanicOnZeroRatio(t *testing.T) {
	assertPanic(t, "requires ratio in (0, 1)", func() {
		WithJitter(NewFixed(time.Second), 0)
	})
}

func TestJitter_PanicOnOneRatio(t *testing.T) {
	assertPanic(t, "requires ratio in (0, 1)", func() {
		WithJitter(NewFixed(time.Second), 1)
	})
}

func TestJitter_PanicOnNegativeRatio(t *testing.T) {
	assertPanic(t, "requires ratio in (0, 1)", func() {
		WithJitter(NewFixed(time.Second), -0.1)
	})
}

func TestJitter_SmallRatio(t *testing.T) {
	inner := NewFixed(time.Second)
	j := WithJitter(inner, 0.01)

	lo := 990 * time.Millisecond
	hi := 1010 * time.Millisecond

	for range 100 {
		got := j.Wait(0)
		if got < lo || got > hi {
			t.Errorf("Wait(0) = %s, want [%s, %s]", got, lo, hi)
		}
	}
}

func TestMaxWait_Truncates(t *testing.T) {
	e := NewExponential(time.Second, 2.0)
	m := WithMaxWait(e, 5*time.Second)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 5 * time.Second}, // would be 8s, capped
		{4, 5 * time.Second}, // would be 16s, capped
	}
	for _, tt := range tests {
		got := m.Wait(tt.attempt)
		if got != tt.expected {
			t.Errorf("Wait(%d) = %s, want %s", tt.attempt, got, tt.expected)
		}
	}
}

func TestMaxWait_PanicOnZeroMax(t *testing.T) {
	assertPanic(t, "requires max > 0", func() {
		WithMaxWait(NewFixed(time.Second), 0)
	})
}

func TestMaxWait_PanicOnNegativeMax(t *testing.T) {
	assertPanic(t, "requires max > 0", func() {
		WithMaxWait(NewFixed(time.Second), -1*time.Second)
	})
}

func TestComposition_ExponentialJitterMaxWait(t *testing.T) {
	e := NewExponential(time.Second, 2.0)
	j := WithJitter(e, 0.1)
	m := WithMaxWait(j, 5*time.Second)

	// attempt=0: base=1s, jitter ±10% → [900ms, 1100ms], max=5s → pass through
	lo0 := 900 * time.Millisecond
	hi0 := 1100 * time.Millisecond

	for range 100 {
		got := m.Wait(0)
		if got < lo0 || got > hi0 {
			t.Errorf("Wait(0) = %s, want [%s, %s]", got, lo0, hi0)
		}
	}

	// attempt=3: inner returns 8s ± jitter, but max at 5s
	got := m.Wait(3)
	if got != 5*time.Second {
		t.Errorf("Wait(3) = %s, want 5s (capped)", got)
	}
}

func TestComposition_FixedJitterMaxWait(t *testing.T) {
	f := NewFixed(2 * time.Second)
	j := WithJitter(f, 0.1)
	m := WithMaxWait(j, 1500*time.Millisecond)

	// Fixed(2s) ± 10% → [1800ms, 2200ms], MaxWait(1500ms) → 1500ms
	for range 50 {
		got := m.Wait(0)
		if got != 1500*time.Millisecond {
			t.Errorf("Wait(0) = %s, want 1500ms", got)
		}
	}
}

func TestExponential_LargeAttempt(t *testing.T) {
	e := NewExponential(time.Millisecond, 2.0)
	got := e.Wait(62)
	// 1ms * 2^62 ≈ 4.6e15 ns, well within time.Duration range
	expected := time.Duration(float64(time.Millisecond) * math.Pow(2, 62))
	if got != expected {
		t.Errorf("Wait(62) = %s, want %s", got, expected)
	}
}

func assertPanic(t *testing.T, wantContains string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s: expected panic", wantContains)
		}
		msg := fmt.Sprintf("%v", r)
		if !strings.Contains(msg, wantContains) {
			t.Fatalf("panic message %q does not contain %q", msg, wantContains)
		}
	}()
	fn()
}
