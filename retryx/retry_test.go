package retryx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// errSentinel is a test error.
var errSentinel = errors.New("sentinel")

func TestDo_SuccessOnFirst(t *testing.T) {
	calls := atomic.Int32{}
	val, err := Do(context.Background(), func() (string, error) {
		calls.Add(1)
		return "ok", nil
	}, Attempts(3))

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if val != "ok" {
		t.Fatalf("expected ok, got %s", val)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", calls.Load())
	}
}

func TestDo_SuccessOnRetry(t *testing.T) {
	calls := atomic.Int32{}
	val, err := Do(context.Background(), func() (int, error) {
		n := calls.Add(1)
		if n < 3 {
			return 0, errSentinel
		}
		return 42, nil
	}, Attempts(3))

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 calls, got %d", calls.Load())
	}
}

func TestDo_AllFail(t *testing.T) {
	calls := atomic.Int32{}
	val, err := Do(context.Background(), func() (string, error) {
		calls.Add(1)
		return "", errSentinel
	}, Attempts(3))

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errSentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if val != "" {
		t.Fatalf("expected zero value, got %s", val)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 calls, got %d", calls.Load())
	}
}

func TestDo_AttemptsOne(t *testing.T) {
	calls := atomic.Int32{}
	_, err := Do(context.Background(), func() (int, error) {
		calls.Add(1)
		return 0, errSentinel
	}, Attempts(1))

	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", calls.Load())
	}
}

func TestDo_RetryIf(t *testing.T) {
	errNotRetryable := errors.New("not retryable")

	calls := atomic.Int32{}
	_, err := Do(context.Background(), func() (int, error) {
		calls.Add(1)
		return 0, errNotRetryable
	},
		Attempts(5),
		RetryIf(func(err error) bool {
			return errors.Is(err, errSentinel)
		}),
	)

	if !errors.Is(err, errNotRetryable) {
		t.Fatalf("expected not-retryable error, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call (no retry), got %d", calls.Load())
	}
}

func TestDo_OnRetry(t *testing.T) {
	var mu sync.Mutex
	var callbacks []struct {
		attempt int
		err     error
	}

	_, _ = Do(context.Background(), func() (int, error) {
		return 0, errSentinel
	},
		Attempts(3),
		OnRetry(func(attempt int, err error) {
			mu.Lock()
			callbacks = append(callbacks, struct {
				attempt int
				err     error
			}{attempt, err})
			mu.Unlock()
		}),
	)

	mu.Lock()
	defer mu.Unlock()
	if len(callbacks) != 2 {
		t.Fatalf("expected 2 callbacks, got %d", len(callbacks))
	}
	for i, cb := range callbacks {
		if cb.attempt != i {
			t.Errorf("callback %d: attempt = %d, want %d", i, cb.attempt, i)
		}
		if !errors.Is(cb.err, errSentinel) {
			t.Errorf("callback %d: want sentinel, got %v", i, cb.err)
		}
	}
}

func TestDo_BackoffExponential(t *testing.T) {
	var mu sync.Mutex
	var timestamps []time.Time

	_, _ = Do(context.Background(), func() (int, error) {
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()
		return 0, errSentinel
	},
		Attempts(4),
		Backoff(NewExponential(50*time.Millisecond, 2.0)),
	)

	mu.Lock()
	defer mu.Unlock()
	if len(timestamps) != 4 {
		t.Fatalf("expected 4 attempts, got %d", len(timestamps))
	}

	// Check gaps: ~50ms, ~100ms, ~200ms (tolerance for CI scheduling jitter)
	expectedGaps := []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}
	for i, gap := range expectedGaps {
		actual := timestamps[i+1].Sub(timestamps[i])
		lo := gap * 2 / 3
		hi := gap * 3
		if actual < lo || actual > hi {
			t.Errorf("gap %d: %s, expected [%s, %s]", i, actual, lo, hi)
		}
	}
}

func TestDo_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := atomic.Int32{}
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := Do(ctx, func() (int, error) {
		calls.Add(1)
		return 0, errSentinel
	},
		Attempts(100),
		Backoff(NewFixed(20*time.Millisecond)),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDo_ContextCancelBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Do(ctx, func() (int, error) {
		return 1, nil
	}, Attempts(3))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDo_GenericTypes(t *testing.T) {
	// int
	intVal, err := Do(context.Background(), func() (int, error) {
		return 42, nil
	})
	if err != nil || intVal != 42 {
		t.Fatalf("int: expected 42, got %d, err=%v", intVal, err)
	}

	// string
	strVal, err := Do(context.Background(), func() (string, error) {
		return "hello", nil
	})
	if err != nil || strVal != "hello" {
		t.Fatalf("string: expected hello, got %s, err=%v", strVal, err)
	}

	// struct
	type point struct{ X, Y int }
	ptVal, err := Do(context.Background(), func() (point, error) {
		return point{1, 2}, nil
	})
	if err != nil || ptVal != (point{1, 2}) {
		t.Fatalf("struct: expected {1,2}, got %+v, err=%v", ptVal, err)
	}
}

func TestDo_Concurrent(t *testing.T) {
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			calls := atomic.Int32{}
			val, err := Do(context.Background(), func() (int, error) {
				n := calls.Add(1)
				if n < 2 {
					return 0, errSentinel
				}
				return 99, nil
			}, Attempts(3))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if val != 99 {
				t.Errorf("expected 99, got %d", val)
			}
		}()
	}
	wg.Wait()
}

func TestAttempts_PanicOnZero(t *testing.T) {
	assertPanic(t, "n must be > 0", func() {
		_, _ = Do(context.Background(), func() (int, error) { return 0, nil }, Attempts(0))
	})
}

func TestAttempts_PanicOnNegative(t *testing.T) {
	assertPanic(t, "n must be > 0", func() {
		_, _ = Do(context.Background(), func() (int, error) { return 0, nil }, Attempts(-1))
	})
}

func TestDo_LastErrorWrapped(t *testing.T) {
	innerErr := errors.New("inner")
	_, err := Do(context.Background(), func() (int, error) {
		return 0, innerErr
	}, Attempts(1))

	if !errors.Is(err, innerErr) {
		t.Fatalf("expected inner error, got %v", err)
	}
	// Verify the wrapper message format.
	want := "retryx: all 1 attempts failed: inner"
	if err.Error() != want {
		t.Fatalf("error message = %q, want %q", err.Error(), want)
	}
}
