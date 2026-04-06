package ctxv_test

import (
	"context"
	"sync"
	"testing"

	"github.com/pinealctx/x/ctxv"
)

func TestWithValueAndValue(t *testing.T) {
	key := ctxv.NewKey[string]("greeting")
	ctx := key.WithValue(context.Background(), "hello")

	val, ok := key.Value(ctx)
	if !ok {
		t.Fatal("expected value to be found")
	}
	if val != "hello" {
		t.Fatalf("expected %q, got %q", "hello", val)
	}
}

func TestValueNotFound(t *testing.T) {
	key := ctxv.NewKey[int]("missing")
	val, ok := key.Value(context.Background())
	if ok {
		t.Fatal("expected value not to be found")
	}
	if val != 0 {
		t.Fatalf("expected zero value 0, got %d", val)
	}
}

func TestMustValueFound(t *testing.T) {
	key := ctxv.NewKey[string]("name")
	ctx := key.WithValue(context.Background(), "alice")

	val := key.MustValue(ctx)
	if val != "alice" {
		t.Fatalf("expected %q, got %q", "alice", val)
	}
}

func TestMustValuePanic(t *testing.T) {
	key := ctxv.NewKey[int]("missing")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if msg != `ctxv: key "missing" not found in context` {
			t.Fatalf("unexpected panic message: %q", msg)
		}
	}()
	key.MustValue(context.Background())
}

func TestDifferentTypesNoInterference(t *testing.T) {
	strKey := ctxv.NewKey[string]("str")
	intKey := ctxv.NewKey[int]("num")

	ctx := strKey.WithValue(context.Background(), "hello")
	ctx = intKey.WithValue(ctx, 42)

	strVal, strOK := strKey.Value(ctx)
	intVal, intOK := intKey.Value(ctx)

	if !strOK || strVal != "hello" {
		t.Fatalf("string key: expected (hello, true), got (%q, %v)", strVal, strOK)
	}
	if !intOK || intVal != 42 {
		t.Fatalf("int key: expected (42, true), got (%d, %v)", intVal, intOK)
	}
}

func TestSameTypeDifferentInstancesNoInterference(t *testing.T) {
	key1 := ctxv.NewKey[string]("first")
	key2 := ctxv.NewKey[string]("second")

	ctx := key1.WithValue(context.Background(), "a")
	ctx = key2.WithValue(ctx, "b")

	val1, ok1 := key1.Value(ctx)
	val2, ok2 := key2.Value(ctx)

	if !ok1 || val1 != "a" {
		t.Fatalf("key1: expected (a, true), got (%q, %v)", val1, ok1)
	}
	if !ok2 || val2 != "b" {
		t.Fatalf("key2: expected (b, true), got (%q, %v)", val2, ok2)
	}
}

func TestEmptyName(t *testing.T) {
	key := ctxv.NewKey[string]("")
	ctx := key.WithValue(context.Background(), "value")

	val, ok := key.Value(ctx)
	if !ok || val != "value" {
		t.Fatalf("expected (value, true), got (%q, %v)", val, ok)
	}
}

func TestString(t *testing.T) {
	key := ctxv.NewKey[int]("request_id")
	if got := key.String(); got != "request_id" {
		t.Fatalf("expected %q, got %q", "request_id", got)
	}
}

func TestWithValueOverwrite(t *testing.T) {
	key := ctxv.NewKey[string]("counter")
	ctx := key.WithValue(context.Background(), "first")
	ctx = key.WithValue(ctx, "second")

	val, ok := key.Value(ctx)
	if !ok || val != "second" {
		t.Fatalf("expected (second, true), got (%q, %v)", val, ok)
	}
}

func TestContextChain(t *testing.T) {
	key := ctxv.NewKey[int]("depth")
	parent := key.WithValue(context.Background(), 100)
	otherKey := ctxv.NewKey[string]("other")
	child := otherKey.WithValue(parent, "data")

	val, ok := key.Value(child)
	if !ok || val != 100 {
		t.Fatalf("expected (100, true), got (%d, %v)", val, ok)
	}
}

func TestZeroValueStored(t *testing.T) {
	intKey := ctxv.NewKey[int]("zero_int")
	ctx := intKey.WithValue(context.Background(), 0)

	val, ok := intKey.Value(ctx)
	if !ok {
		t.Fatal("expected zero value to be found")
	}
	if val != 0 {
		t.Fatalf("expected 0, got %d", val)
	}

	strKey := ctxv.NewKey[string]("empty_str")
	ctx = strKey.WithValue(context.Background(), "")

	sVal, sOK := strKey.Value(ctx)
	if !sOK {
		t.Fatal("expected empty string to be found")
	}
	if sVal != "" {
		t.Fatalf("expected empty string, got %q", sVal)
	}
}

func TestNilContextPanics(t *testing.T) {
	key := ctxv.NewKey[string]("nil_ctx")
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic with nil context")
		}
	}()
	key.Value(nil) //nolint:staticcheck // nil context is intentional for this test
}

func TestConcurrentAccess(t *testing.T) {
	key := ctxv.NewKey[int]("concurrent")
	ctx := key.WithValue(context.Background(), 42)

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			val, ok := key.Value(ctx)
			if !ok || val != 42 {
				t.Errorf("expected (42, true), got (%d, %v)", val, ok)
			}
		})
	}
	wg.Wait()
}
