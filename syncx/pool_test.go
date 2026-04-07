package syncx_test

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pinealctx/x/syncx"
)

// --- Tests ---

func TestPool_GetFromEmptyCallsCreate(t *testing.T) {
	var creates atomic.Int32
	p := syncx.NewPool[int](func() int {
		creates.Add(1)
		return 42
	})

	v := p.Get()
	if v != 42 {
		t.Fatalf("expected 42, got %d", v)
	}
	if creates.Load() != 1 {
		t.Fatalf("expected 1 create call, got %d", creates.Load())
	}
}

func TestPool_PutThenGet(_ *testing.T) {
	p := syncx.NewPool[*bytes.Buffer](func() *bytes.Buffer {
		return new(bytes.Buffer)
	})

	buf := p.Get()
	buf.WriteString("hello")
	p.Put(buf)

	// sync.Pool does not guarantee Put-then-Get returns the same object
	// (GC may clear the pool at any time). Just verify Get does not panic.
	_ = p.Get()
}

func TestPool_ResetCalledOnPut(t *testing.T) {
	var resets atomic.Int32
	p := syncx.NewPool[*bytes.Buffer](
		func() *bytes.Buffer {
			return new(bytes.Buffer)
		},
		func(b *bytes.Buffer) {
			resets.Add(1)
			b.Reset()
		},
	)

	buf := p.Get()
	buf.WriteString("data")
	p.Put(buf)

	if resets.Load() != 1 {
		t.Fatalf("expected 1 reset call, got %d", resets.Load())
	}
}

func TestPool_NoResetNoPanic(_ *testing.T) {
	p := syncx.NewPool[int](func() int {
		return 99
	})

	p.Put(99) // no reset — should not panic.
	_ = p.Get()
}

func TestPool_ByteBufferType(t *testing.T) {
	var resets atomic.Int32
	p := syncx.NewPool[*bytes.Buffer](
		func() *bytes.Buffer {
			return new(bytes.Buffer)
		},
		func(b *bytes.Buffer) {
			resets.Add(1)
			b.Reset()
		},
	)

	buf := p.Get()
	buf.WriteString("hello")
	p.Put(buf)

	// Verify reset callback was invoked; Len() check is unreliable because
	// GC may clear the pool and create returns a fresh buffer either way.
	if resets.Load() != 1 {
		t.Fatalf("expected 1 reset call, got %d", resets.Load())
	}
}

type testItem struct {
	ID   int
	Name string
}

func TestPool_CustomStructType(t *testing.T) {
	p := syncx.NewPool[testItem](func() testItem {
		return testItem{ID: 1, Name: "default"}
	})

	v := p.Get()
	if v.ID != 1 || v.Name != "default" {
		t.Fatalf("expected {1, default}, got {%d, %s}", v.ID, v.Name)
	}

	// Put and Get exercise the value-type path through the generic wrapper.
	// sync.Pool may clear entries, so we only verify no panic.
	p.Put(testItem{ID: 2, Name: "modified"})
	_ = p.Get()
}

func TestPool_IntType(t *testing.T) {
	// Verify that value types (int) work correctly through the generic wrapper.
	// Only the first Get is tested: sync.Pool does not guarantee subsequent
	// Gets will miss the pool, so a strict sequence assertion is not valid.
	var creates atomic.Int32
	p := syncx.NewPool[int](func() int {
		return int(creates.Add(1))
	})

	v := p.Get()
	if v != 1 {
		t.Fatalf("first Get: expected 1, got %d", v)
	}
	if creates.Load() != 1 {
		t.Fatalf("expected 1 create call, got %d", creates.Load())
	}
}

func TestPool_ConcurrentStress(t *testing.T) {
	var creates atomic.Int32
	p := syncx.NewPool[int](
		func() int {
			creates.Add(1)
			return 0
		},
	)

	const goroutines = 100
	const opsPerGoroutine = 50

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				v := p.Get()
				// Use the value to prevent optimization.
				v = v + idx + j
				p.Put(v)
			}
		}(i)
	}
	wg.Wait()

	if creates.Load() == 0 {
		t.Fatal("expected at least one create call")
	}
}

func TestPool_ConcurrentGetPut(t *testing.T) {
	p := syncx.NewPool[*bytes.Buffer](
		func() *bytes.Buffer {
			return new(bytes.Buffer)
		},
		func(b *bytes.Buffer) { b.Reset() },
	)

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	var errors atomic.Int32
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				buf := p.Get()
				if buf == nil {
					errors.Add(1)
					continue
				}
				buf.WriteString("goroutine data")
				p.Put(buf)
			}
		}()
	}
	wg.Wait()

	if n := errors.Load(); n > 0 {
		t.Fatalf("got %d nil buffers during concurrent Get/Put", n)
	}
}

func TestPool_PanicOnNilCreate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil create function")
		}
	}()
	syncx.NewPool[int](nil)
}

func TestPool_PanicOnMultipleResets(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for multiple reset functions")
		}
	}()
	syncx.NewPool[int](
		func() int { return 0 },
		func(int) {},
		func(int) {},
	)
}

func TestPool_ZeroValueCreate(t *testing.T) {
	// Verify that create returning the zero value of T works correctly.
	p := syncx.NewPool[*bytes.Buffer](func() *bytes.Buffer {
		return nil // zero value for *bytes.Buffer
	})

	v := p.Get()
	if v != nil {
		t.Fatalf("expected nil, got %v", v)
	}

	// Put nil back — should not panic.
	p.Put(nil)
}
