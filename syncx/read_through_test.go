package syncx_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pinealctx/x/syncx"
)

// --- Test helpers ---

// mockCache is a thread-safe in-memory Cache[K,V] for testing.
type mockCache[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

func newMockCache[K comparable, V any]() *mockCache[K, V] {
	return &mockCache[K, V]{data: make(map[K]V)}
}

func (c *mockCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	v, ok := c.data[key]
	c.mu.RUnlock()
	return v, ok
}

func (c *mockCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.data[key] = value
	c.mu.Unlock()
}

// syncMapCache is an alternative Cache implementation backed by sync.Map,
// used to verify that third-party cache adapters work with ReadThrough.
type syncMapCache[K comparable, V any] struct {
	m sync.Map
}

func (c *syncMapCache[K, V]) Get(key K) (V, bool) {
	v, ok := c.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	typed, _ := v.(V)
	return typed, true
}

func (c *syncMapCache[K, V]) Set(key K, value V) {
	c.m.Store(key, value)
}

// --- Tests ---

func TestReadThrough_CacheHit(t *testing.T) {
	c := newMockCache[string, int]()
	c.Set("key", 42)

	var loaderCalls atomic.Int32
	rt := syncx.NewReadThrough[string, int](c, func(_ context.Context, _ string) (int, error) {
		loaderCalls.Add(1)
		return 99, nil
	})

	v, err := rt.Get(context.Background(), "key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != 42 {
		t.Fatalf("expected 42, got %d", v)
	}
	if loaderCalls.Load() != 0 {
		t.Fatalf("loader should not be called on cache hit, got %d calls", loaderCalls.Load())
	}
}

func TestReadThrough_CacheMiss_LoadAndPopulate(t *testing.T) {
	c := newMockCache[string, int]()
	rt := syncx.NewReadThrough[string, int](c, func(_ context.Context, key string) (int, error) {
		return len(key), nil
	})

	v, err := rt.Get(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != 5 {
		t.Fatalf("expected 5, got %d", v)
	}

	// Verify cache was populated.
	if cached, ok := c.Get("hello"); !ok || cached != 5 {
		t.Fatalf("cache not populated: got %d, %v", cached, ok)
	}
}

func TestReadThrough_StampedeProtection(t *testing.T) {
	c := newMockCache[string, int]()

	var loaderCalls atomic.Int32
	rt := syncx.NewReadThrough[string, int](c, func(_ context.Context, _ string) (int, error) {
		loaderCalls.Add(1)
		time.Sleep(50 * time.Millisecond) // slow load
		return 77, nil
	})

	const n = 20
	var wg sync.WaitGroup
	results := make([]int, n)
	errors := make([]error, n)

	// Barrier to ensure all goroutines start together.
	var startWg sync.WaitGroup
	startWg.Add(1)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			startWg.Wait() // wait for barrier
			v, err := rt.Get(context.Background(), "key")
			results[idx] = v
			errors[idx] = err
		}(i)
	}

	startWg.Done() // release all goroutines
	wg.Wait()

	for i := range n {
		if errors[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errors[i])
		}
		if results[i] != 77 {
			t.Fatalf("goroutine %d: expected 77, got %d", i, results[i])
		}
	}

	if calls := loaderCalls.Load(); calls != 1 {
		t.Fatalf("expected exactly 1 loader call, got %d", calls)
	}
}

func TestReadThrough_DifferentKeysConcurrent(t *testing.T) {
	c := newMockCache[string, int]()

	var loadOrder []string
	var orderMu sync.Mutex
	rt := syncx.NewReadThrough[string, int](c, func(_ context.Context, key string) (int, error) {
		orderMu.Lock()
		loadOrder = append(loadOrder, key)
		orderMu.Unlock()
		time.Sleep(50 * time.Millisecond) // slow enough to overlap
		return len(key), nil
	})

	var wg sync.WaitGroup
	var resultA, resultB int
	var errA, errB error

	wg.Add(2)
	go func() {
		defer wg.Done()
		resultA, errA = rt.Get(context.Background(), "a")
	}()
	go func() {
		defer wg.Done()
		resultB, errB = rt.Get(context.Background(), "bb")
	}()
	wg.Wait()

	if errA != nil || errB != nil {
		t.Fatalf("errors: %v, %v", errA, errB)
	}
	if resultA != 1 {
		t.Fatalf("key 'a': expected 1, got %d", resultA)
	}
	if resultB != 2 {
		t.Fatalf("key 'bb': expected 2, got %d", resultB)
	}

	// Both keys should have been loaded (no blocking between different keys).
	orderMu.Lock()
	if len(loadOrder) != 2 {
		t.Fatalf("expected 2 loads, got %d", len(loadOrder))
	}
	orderMu.Unlock()
}

func TestReadThrough_LoaderError(t *testing.T) {
	c := newMockCache[string, int]()
	loadErr := errors.New("load failed")

	var calls atomic.Int32
	rt := syncx.NewReadThrough[string, int](c, func(_ context.Context, _ string) (int, error) {
		calls.Add(1)
		return 0, loadErr
	})

	// First call: error returned.
	_, err := rt.Get(context.Background(), "key")
	if !errors.Is(err, loadErr) {
		t.Fatalf("expected loadErr, got %v", err)
	}

	// Error not cached — second call invokes loader again.
	_, err = rt.Get(context.Background(), "key")
	if !errors.Is(err, loadErr) {
		t.Fatalf("expected loadErr on retry, got %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 loader calls (error not cached), got %d", calls.Load())
	}
}

func TestReadThrough_ContextCancellation(t *testing.T) {
	c := newMockCache[string, int]()
	rt := syncx.NewReadThrough[string, int](c, func(ctx context.Context, _ string) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := rt.Get(ctx, "key")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestReadThrough_ThirdPartyCacheAdapter(t *testing.T) {
	c := &syncMapCache[string, int]{}
	rt := syncx.NewReadThrough[string, int](c, func(_ context.Context, key string) (int, error) {
		return len(key) * 10, nil
	})

	v, err := rt.Get(context.Background(), "abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != 30 {
		t.Fatalf("expected 30, got %d", v)
	}

	// Verify syncMapCache was populated.
	if cached, ok := c.Get("abc"); !ok || cached != 30 {
		t.Fatalf("cache not populated: got %d, %v", cached, ok)
	}
}

func TestReadThrough_ConcurrentStress(t *testing.T) {
	c := newMockCache[int, int]()

	const keys = 10
	const goroutines = 200

	var loaderCalls atomic.Int32
	rt := syncx.NewReadThrough[int, int](c, func(_ context.Context, key int) (int, error) {
		loaderCalls.Add(1)
		time.Sleep(time.Millisecond) // small delay to increase contention
		return key * 7, nil
	})

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := idx % keys
			v, err := rt.Get(context.Background(), key)
			if err != nil {
				t.Errorf("goroutine %d: %v", idx, err)
				return
			}
			if v != key*7 {
				t.Errorf("goroutine %d key %d: expected %d, got %d", idx, key, key*7, v)
			}
		}(i)
	}
	wg.Wait()

	if calls := loaderCalls.Load(); calls > keys {
		t.Fatalf("expected at most %d loader calls, got %d", keys, calls)
	}
}

func TestReadThrough_PanicOnNilCache(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil cache")
		}
	}()
	syncx.NewReadThrough[string, int](nil, func(_ context.Context, _ string) (int, error) {
		return 0, nil
	})
}

func TestReadThrough_PanicOnNilLoader(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil loader")
		}
	}()
	c := newMockCache[string, int]()
	syncx.NewReadThrough[string, int](c, nil)
}

func TestReadThrough_LoaderPanic(t *testing.T) {
	c := newMockCache[string, int]()
	var calls atomic.Int32
	rt := syncx.NewReadThrough[string, int](c, func(_ context.Context, _ string) (int, error) {
		if calls.Add(1) == 1 {
			panic("boom")
		}
		return 42, nil
	})

	// First call: panic propagates, but per-key lock is released via defer.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic from loader")
			}
		}()
		_, _ = rt.Get(context.Background(), "key")
	}()

	// Second call on the same key must not deadlock — proves defer unlock() ran.
	v, err := rt.Get(context.Background(), "key")
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if v != 42 {
		t.Fatalf("expected 42, got %d", v)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 loader calls, got %d", calls.Load())
	}
}

func TestReadThrough_LoaderErrorConcurrent(t *testing.T) {
	c := newMockCache[string, int]()
	loadErr := errors.New("load failed")

	var loaderCalls atomic.Int32
	rt := syncx.NewReadThrough[string, int](c, func(_ context.Context, _ string) (int, error) {
		loaderCalls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return 0, loadErr
	})

	const n = 10
	var wg sync.WaitGroup
	var startWg sync.WaitGroup
	startWg.Add(1)

	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			startWg.Wait()
			_, errs[idx] = rt.Get(context.Background(), "key")
		}(i)
	}

	startWg.Done()
	wg.Wait()

	for i, err := range errs {
		if !errors.Is(err, loadErr) {
			t.Fatalf("goroutine %d: expected loadErr, got %v", i, err)
		}
	}

	// Errors are not cached, so each goroutine that acquires the lock finds a
	// cache miss on double-check and calls the loader independently.
	if calls := loaderCalls.Load(); calls != n {
		t.Fatalf("expected %d loader calls (errors not cached, each holder retries), got %d", n, calls)
	}
}

func TestReadThrough_ZeroValueLoad(t *testing.T) {
	c := newMockCache[string, int]()
	rt := syncx.NewReadThrough[string, int](c, func(_ context.Context, _ string) (int, error) {
		return 0, nil // zero value of V=int
	})

	v, err := rt.Get(context.Background(), "key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != 0 {
		t.Fatalf("expected 0, got %d", v)
	}

	// Verify cache was populated with the zero value.
	cached, ok := c.Get("key")
	if !ok {
		t.Fatal("expected cache hit for zero value")
	}
	if cached != 0 {
		t.Fatalf("expected cached 0, got %d", cached)
	}
}
