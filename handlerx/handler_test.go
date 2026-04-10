package handlerx_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/pinealctx/x/handlerx"
)

func echo(_ context.Context, req string) (string, error) {
	return req, nil
}

func failHandler(_ context.Context, _ string) (string, error) {
	return "", errors.New("handler error")
}

func TestChain_NoInterceptors(t *testing.T) {
	h := handlerx.Chain(echo)
	resp, err := h(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "hello" {
		t.Fatalf("got %q, want %q", resp, "hello")
	}
}

func TestChain_SingleInterceptor(t *testing.T) {
	var log []string
	h := handlerx.Chain(
		func(_ context.Context, req string) (string, error) {
			log = append(log, "handler")
			return req, nil
		},
		func(ctx context.Context, req string, next handlerx.Handler[string, string]) (string, error) {
			log = append(log, "i0:enter")
			resp, err := next(ctx, req)
			log = append(log, "i0:exit")
			return resp, err
		},
	)

	_, err := h(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"i0:enter", "handler", "i0:exit"}
	if len(log) != len(want) {
		t.Fatalf("log = %v, want %v", log, want)
	}
	for i, v := range want {
		if log[i] != v {
			t.Fatalf("log[%d] = %q, want %q", i, log[i], v)
		}
	}
}

func TestChain_Order(t *testing.T) {
	// Expected order: i0 enter → i1 enter → handler → i1 exit → i0 exit
	var log []string

	makeInterceptor := func(label string) handlerx.Interceptor[string, string] {
		return func(ctx context.Context, req string, next handlerx.Handler[string, string]) (string, error) {
			log = append(log, label+":enter")
			resp, err := next(ctx, req)
			log = append(log, label+":exit")
			return resp, err
		}
	}

	h := handlerx.Chain(
		func(_ context.Context, req string) (string, error) {
			log = append(log, "handler")
			return req, nil
		},
		makeInterceptor("i0"),
		makeInterceptor("i1"),
	)

	_, err := h(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"i0:enter", "i1:enter", "handler", "i1:exit", "i0:exit"}
	if len(log) != len(want) {
		t.Fatalf("log = %v, want %v", log, want)
	}
	for i, v := range want {
		if log[i] != v {
			t.Fatalf("log[%d] = %q, want %q", i, log[i], v)
		}
	}
}

func TestChain_InterceptorCanShortCircuit(t *testing.T) {
	sentinel := errors.New("short circuit")
	guard := func(_ context.Context, _ string, _ handlerx.Handler[string, string]) (string, error) {
		return "", sentinel
	}

	called := false
	h := handlerx.Chain(
		func(_ context.Context, req string) (string, error) {
			called = true
			return req, nil
		},
		guard,
	)

	_, err := h(context.Background(), "x")
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v, want sentinel", err)
	}
	if called {
		t.Fatal("handler should not have been called")
	}
}

func TestChain_ErrorPropagates(t *testing.T) {
	h := handlerx.Chain(failHandler)
	_, err := h(context.Background(), "x")
	if err == nil || err.Error() != "handler error" {
		t.Fatalf("got %v, want handler error", err)
	}
}

func TestChain_MultipleCallsAreIndependent(t *testing.T) {
	h := handlerx.Chain(echo)
	for range 5 {
		resp, err := h(context.Background(), "ping")
		if err != nil || resp != "ping" {
			t.Fatalf("unexpected result: (%q, %v)", resp, err)
		}
	}
}

func TestChain_ThreeInterceptorsOrder(t *testing.T) {
	// Verify iterative chain building is correct for n=3.
	// Expected: i0:enter → i1:enter → i2:enter → handler → i2:exit → i1:exit → i0:exit
	var log []string

	makeInterceptor := func(label string) handlerx.Interceptor[string, string] {
		return func(ctx context.Context, req string, next handlerx.Handler[string, string]) (string, error) {
			log = append(log, label+":enter")
			resp, err := next(ctx, req)
			log = append(log, label+":exit")
			return resp, err
		}
	}

	h := handlerx.Chain(
		func(_ context.Context, req string) (string, error) {
			log = append(log, "handler")
			return req, nil
		},
		makeInterceptor("i0"),
		makeInterceptor("i1"),
		makeInterceptor("i2"),
	)

	_, err := h(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"i0:enter", "i1:enter", "i2:enter", "handler", "i2:exit", "i1:exit", "i0:exit"}
	if len(log) != len(want) {
		t.Fatalf("log = %v, want %v", log, want)
	}
	for i, v := range want {
		if log[i] != v {
			t.Fatalf("log[%d] = %q, want %q", i, log[i], v)
		}
	}
}

func TestChain_ConcurrentCallsSafe(t *testing.T) {
	// Verify that a single Handler instance is safe for concurrent use.
	h := handlerx.Chain(echo)
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			resp, err := h(context.Background(), "ping")
			if err != nil || resp != "ping" {
				t.Errorf("unexpected result: (%q, %v)", resp, err)
			}
		}()
	}
	wg.Wait()
}
