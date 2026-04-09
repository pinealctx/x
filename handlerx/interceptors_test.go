package handlerx_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pinealctx/x/handlerx"
)

// --- WithTimeout ---

func TestWithTimeout_Completes(t *testing.T) {
	h := handlerx.Chain(echo, handlerx.WithTimeout[string, string](100*time.Millisecond))
	resp, err := h(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "hi" {
		t.Fatalf("got %q, want %q", resp, "hi")
	}
}

func TestWithTimeout_Expires(t *testing.T) {
	slow := func(ctx context.Context, _ string) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
			return "done", nil
		}
	}

	h := handlerx.Chain(slow, handlerx.WithTimeout[string, string](10*time.Millisecond))
	_, err := h(context.Background(), "x")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want DeadlineExceeded", err)
	}
}

func TestWithTimeout_ZeroDuration(t *testing.T) {
	// context.WithTimeout(ctx, 0) creates an already-expired context.
	slow := func(ctx context.Context, _ string) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
			return "done", nil
		}
	}

	h := handlerx.Chain(slow, handlerx.WithTimeout[string, string](0))
	_, err := h(context.Background(), "x")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want DeadlineExceeded", err)
	}
}

func TestWithTimeout_RespectsParentCancellation(t *testing.T) {
	slow := func(ctx context.Context, _ string) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
			return "done", nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	h := handlerx.Chain(slow, handlerx.WithTimeout[string, string](10*time.Second))

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := h(ctx, "x")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want Canceled", err)
	}
}

func TestWithRecovery_PanicReturnsZeroResp(t *testing.T) {
	// WithRecovery uses named returns; on panic, resp must be the zero value.
	panicker := func(_ context.Context, _ string) (string, error) {
		panic("boom")
	}

	h := handlerx.Chain(panicker, handlerx.WithRecovery[string, string]())
	resp, err := h(context.Background(), "x")
	if !errors.Is(err, handlerx.ErrPanicRecovered) {
		t.Fatalf("got %v, want ErrPanicRecovered", err)
	}
	if resp != "" {
		t.Fatalf("resp = %q, want zero value", resp)
	}
}

func TestWithTimeout_ContextCanceledAfterCall(t *testing.T) {
	// Verify that the context derived by WithTimeout is canceled after the
	// handler returns, confirming defer cancel() runs and resources are released.
	ctxCh := make(chan context.Context, 1)
	h := handlerx.Chain(
		func(ctx context.Context, req string) (string, error) {
			ctxCh <- ctx
			return req, nil
		},
		handlerx.WithTimeout[string, string](100*time.Millisecond),
	)

	_, err := h(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After Chain returns, defer cancel() has run; the derived context must be done.
	capturedCtx := <-ctxCh
	if capturedCtx.Err() == nil {
		t.Fatal("expected derived context to be canceled after handler returned")
	}
}

// --- WithRecovery ---

func TestWithRecovery_NoPanic(t *testing.T) {
	h := handlerx.Chain(echo, handlerx.WithRecovery[string, string]())
	resp, err := h(context.Background(), "ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("got %q, want %q", resp, "ok")
	}
}

func TestWithRecovery_CatchesPanic(t *testing.T) {
	panicker := func(_ context.Context, _ string) (string, error) {
		panic("something went wrong")
	}

	h := handlerx.Chain(panicker, handlerx.WithRecovery[string, string]())
	_, err := h(context.Background(), "x")
	if !errors.Is(err, handlerx.ErrPanicRecovered) {
		t.Fatalf("got %v, want ErrPanicRecovered", err)
	}
}

func TestWithRecovery_CatchesPanicValue(t *testing.T) {
	panicker := func(_ context.Context, _ string) (string, error) {
		panic(42)
	}

	h := handlerx.Chain(panicker, handlerx.WithRecovery[string, string]())
	_, err := h(context.Background(), "x")
	if !errors.Is(err, handlerx.ErrPanicRecovered) {
		t.Fatalf("got %v, want ErrPanicRecovered", err)
	}
}

func TestWithRecovery_CatchesPanicNil(t *testing.T) {
	// panic(nil) in Go 1.21+ is recovered as *runtime.PanicNilError (non-nil).
	panicker := func(_ context.Context, _ string) (string, error) {
		panic(nil) //nolint:govet // intentional panic(nil) boundary test
	}

	h := handlerx.Chain(panicker, handlerx.WithRecovery[string, string]())
	_, err := h(context.Background(), "x")
	if !errors.Is(err, handlerx.ErrPanicRecovered) {
		t.Fatalf("got %v, want ErrPanicRecovered", err)
	}
}

func TestWithRecovery_ErrorFromHandlerPassesThrough(t *testing.T) {
	h := handlerx.Chain(failHandler, handlerx.WithRecovery[string, string]())
	_, err := h(context.Background(), "x")
	if err == nil || err.Error() != "handler error" {
		t.Fatalf("got %v, want handler error", err)
	}
}

// --- Combined ---

func TestChain_TimeoutOuterRecoveryInner(t *testing.T) {
	// WithTimeout outermost, WithRecovery inner — panic is caught by Recovery
	// before Timeout can observe it.
	panicker := func(_ context.Context, _ string) (string, error) {
		panic("boom")
	}

	h := handlerx.Chain(
		panicker,
		handlerx.WithTimeout[string, string](100*time.Millisecond),
		handlerx.WithRecovery[string, string](),
	)
	_, err := h(context.Background(), "x")
	if !errors.Is(err, handlerx.ErrPanicRecovered) {
		t.Fatalf("got %v, want ErrPanicRecovered", err)
	}
}

func TestChain_RecoveryOuterTimeoutInner(t *testing.T) {
	// WithRecovery outermost, WithTimeout inner — panic inside timeout window.
	panicker := func(_ context.Context, _ string) (string, error) {
		panic("boom")
	}

	h := handlerx.Chain(
		panicker,
		handlerx.WithRecovery[string, string](),
		handlerx.WithTimeout[string, string](100*time.Millisecond),
	)
	_, err := h(context.Background(), "x")
	if !errors.Is(err, handlerx.ErrPanicRecovered) {
		t.Fatalf("got %v, want ErrPanicRecovered", err)
	}
}
