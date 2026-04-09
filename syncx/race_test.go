package syncx

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRace_Empty(t *testing.T) {
	val, err := Race[int](context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 0 {
		t.Fatalf("got %d, want 0", val)
	}
}

func TestRace_SingleSuccess(t *testing.T) {
	val, err := Race(context.Background(), func(_ context.Context) (int, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Fatalf("got %d, want 42", val)
	}
}

func TestRace_SingleFailure(t *testing.T) {
	want := errors.New("fail")
	_, err := Race(context.Background(), func(_ context.Context) (int, error) {
		return 0, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}

func TestRace_FirstSuccessWins(t *testing.T) {
	// fn[0] succeeds immediately; fn[1] blocks until its ctx is canceled.
	val, err := Race(context.Background(),
		func(_ context.Context) (int, error) {
			return 1, nil
		},
		func(ctx context.Context) (int, error) {
			<-ctx.Done()
			return 0, ctx.Err()
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 1 {
		t.Fatalf("got %d, want 1", val)
	}
}

func TestRace_SlowSuccessWins(t *testing.T) {
	// fn[0] fails fast; fn[1] waits for fn[0] to fail before succeeding,
	// eliminating timing dependency.
	fn0done := make(chan struct{})
	val, err := Race(context.Background(),
		func(_ context.Context) (int, error) {
			defer close(fn0done)
			return 0, errors.New("fast fail")
		},
		func(ctx context.Context) (int, error) {
			select {
			case <-fn0done:
			case <-ctx.Done():
				return 0, ctx.Err()
			}
			return 99, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 99 {
		t.Fatalf("got %d, want 99", val)
	}
}

func TestRace_AllFail(t *testing.T) {
	err1 := errors.New("err1")
	err2 := errors.New("err2")
	err3 := errors.New("err3")

	_, err := Race(context.Background(),
		func(_ context.Context) (int, error) { return 0, err1 },
		func(_ context.Context) (int, error) { return 0, err2 },
		func(_ context.Context) (int, error) { return 0, err3 },
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// lastErr is the last error written by any goroutine (scheduling order, not
	// submission order), so we only assert it is one of the three.
	if !errors.Is(err, err1) && !errors.Is(err, err2) && !errors.Is(err, err3) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRace_ContextCanceledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	_, err := Race(ctx, func(ctx context.Context) (int, error) {
		return 0, ctx.Err()
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRace_ExternalContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		_, err := Race(ctx,
			func(ctx context.Context) (int, error) {
				close(started)
				<-ctx.Done()
				return 0, ctx.Err()
			},
			func(ctx context.Context) (int, error) {
				<-ctx.Done()
				return 0, ctx.Err()
			},
		)
		done <- err
	}()

	// Cancel after the first fn has started.
	<-started
	cancel()

	err := <-done
	if err == nil {
		t.Fatal("expected error after context cancel")
	}
}

func TestRace_SuccessCancelsOthers(t *testing.T) {
	// Verify that the context passed to remaining fns is canceled after a winner.
	// Race waits for all goroutines before returning, so canceled must be true
	// by the time Race returns.
	var canceled atomic.Bool

	_, err := Race(context.Background(),
		func(_ context.Context) (int, error) {
			return 7, nil
		},
		func(ctx context.Context) (int, error) {
			<-ctx.Done()
			canceled.Store(true)
			return 0, ctx.Err()
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !canceled.Load() {
		t.Fatal("second fn context was not canceled after winner")
	}
}

func TestRace_NoGoroutineLeak(t *testing.T) {
	// All fns block until their ctx is canceled; one succeeds immediately.
	// After Race returns, all goroutines must have exited.
	var running atomic.Int32

	_, err := Race(context.Background(),
		func(_ context.Context) (int, error) {
			return 1, nil
		},
		func(ctx context.Context) (int, error) {
			running.Add(1)
			defer running.Add(-1)
			<-ctx.Done()
			return 0, ctx.Err()
		},
		func(ctx context.Context) (int, error) {
			running.Add(1)
			defer running.Add(-1)
			<-ctx.Done()
			return 0, ctx.Err()
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Race waits for all goroutines; running must be 0 here.
	if n := running.Load(); n != 0 {
		t.Fatalf("goroutine leak: %d goroutines still running", n)
	}
}

func TestRace_Concurrent(t *testing.T) {
	// Stress: many concurrent Race calls, each with multiple fns.
	const goroutines = 50
	errs := make(chan error, goroutines)

	for range goroutines {
		go func() {
			val, err := Race(context.Background(),
				func(_ context.Context) (int, error) {
					time.Sleep(time.Millisecond)
					return 1, nil
				},
				func(_ context.Context) (int, error) {
					time.Sleep(2 * time.Millisecond)
					return 2, nil
				},
			)
			if err != nil {
				errs <- err
				return
			}
			if val != 1 && val != 2 {
				errs <- errors.New("unexpected value")
				return
			}
			errs <- nil
		}()
	}

	for range goroutines {
		if err := <-errs; err != nil {
			t.Errorf("concurrent Race error: %v", err)
		}
	}
}

func TestRace_SuccessWithPointerType(t *testing.T) {
	// Verify that a non-scalar success result (pointer) is returned correctly.
	type S struct{ X int }
	val, err := Race(context.Background(), func(_ context.Context) (*S, error) {
		return &S{X: 5}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil || val.X != 5 {
		t.Fatalf("unexpected value: %v", val)
	}
}

func TestRace_ZeroValueOnAllFail(t *testing.T) {
	// When all fail, returned value must be zero.
	val, err := Race(context.Background(), func(_ context.Context) (int, error) {
		return 99, errors.New("fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if val != 0 {
		t.Fatalf("got %d, want 0 on failure", val)
	}
}

func TestRace_SuccessWithZeroValue(t *testing.T) {
	// Verify that a successful fn returning the zero value is correctly reported
	// as success (err == nil), not confused with the all-fail path.
	val, err := Race(context.Background(), func(_ context.Context) (int, error) {
		return 0, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 0 {
		t.Fatalf("got %d, want 0", val)
	}
}
