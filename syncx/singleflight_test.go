package syncx

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleFlight_BasicDo(t *testing.T) {
	sf := NewSingleFlight[string, int]()
	val, shared, err := sf.Do("key", func() (int, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Fatalf("got %d, want 42", val)
	}
	if shared {
		t.Fatal("shared should be false for the executor")
	}
}

func TestSingleFlight_ConcurrentDedup(t *testing.T) {
	sf := NewSingleFlight[string, int]()
	var execCount atomic.Int32

	const N = 100
	var wg sync.WaitGroup
	type result struct {
		val    int
		shared bool
		err    error
	}
	results := make([]result, N)
	start := make(chan struct{})

	for i := range N {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			v, s, e := sf.Do("key", func() (int, error) {
				execCount.Add(1)
				time.Sleep(50 * time.Millisecond)
				return 42, nil
			})
			results[idx] = result{v, s, e}
		}(i)
	}

	close(start)
	wg.Wait()

	if got := execCount.Load(); got != 1 {
		t.Fatalf("fn executed %d times, want 1", got)
	}

	executors := 0
	for i, r := range results {
		if r.err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, r.err)
		}
		if r.val != 42 {
			t.Errorf("goroutine %d: got %d, want 42", i, r.val)
		}
		if !r.shared {
			executors++
		}
	}
	if executors != 1 {
		t.Errorf("expected exactly 1 executor, got %d", executors)
	}
}

func TestSingleFlight_DifferentKeysIndependent(t *testing.T) {
	sf := NewSingleFlight[int, int]()
	var execCount atomic.Int32

	const N = 10
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := range N {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			v, shared, err := sf.Do(idx, func() (int, error) {
				execCount.Add(1)
				time.Sleep(10 * time.Millisecond)
				return idx * 10, nil
			})
			if err != nil {
				t.Errorf("key %d: %v", idx, err)
			}
			if v != idx*10 {
				t.Errorf("key %d: got %d, want %d", idx, v, idx*10)
			}
			if shared {
				t.Errorf("key %d: shared should be false", idx)
			}
		}(i)
	}

	close(start)
	wg.Wait()

	if got := execCount.Load(); got != N {
		t.Fatalf("fn executed %d times, want %d", got, N)
	}
}

func TestSingleFlight_ErrorPropagation(t *testing.T) {
	sf := NewSingleFlight[string, int]()
	testErr := errors.New("test error")

	const N = 10
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, N)

	for i := range N {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			_, _, errs[idx] = sf.Do("key", func() (int, error) {
				return 0, testErr
			})
		}(i)
	}

	close(start)
	wg.Wait()

	for i, e := range errs {
		if e == nil {
			t.Errorf("goroutine %d: expected error, got nil", i)
		}
		if !errors.Is(e, testErr) {
			t.Errorf("goroutine %d: got %v, want %v", i, e, testErr)
		}
	}
}

func TestSingleFlight_Forget(t *testing.T) {
	sf := NewSingleFlight[string, int]()
	var execCount atomic.Int32

	fn := func() (int, error) {
		return int(execCount.Add(1)), nil
	}

	v1, shared1, err := sf.Do("key", fn)
	if err != nil {
		t.Fatalf("first Do: %v", err)
	}
	if v1 != 1 {
		t.Fatalf("first Do: got %d, want 1", v1)
	}
	if shared1 {
		t.Fatal("first Do: shared should be false")
	}

	// After Do completes, key is already removed from map.
	// Forget is a no-op but should not panic.
	sf.Forget("key")

	v2, shared2, err := sf.Do("key", fn)
	if err != nil {
		t.Fatalf("second Do: %v", err)
	}
	if v2 != 2 {
		t.Fatalf("second Do: got %d, want 2", v2)
	}
	if shared2 {
		t.Fatal("second Do: shared should be false")
	}

	if got := execCount.Load(); got != 2 {
		t.Fatalf("fn executed %d times, want 2", got)
	}
}

func TestSingleFlight_ForgetDuringFlight(t *testing.T) {
	sf := NewSingleFlight[string, int]()
	var execCount atomic.Int32
	started := make(chan struct{})
	unblock := make(chan struct{})

	var firstDone sync.WaitGroup
	firstDone.Add(1)

	// Start a goroutine that holds an in-flight call.
	go func() {
		defer firstDone.Done()
		v, shared, err := sf.Do("key", func() (int, error) {
			close(started)
			<-unblock
			execCount.Add(1)
			return 1, nil
		})
		if err != nil {
			t.Errorf("first Do: %v", err)
		}
		if v != 1 {
			t.Errorf("first Do: got %d, want 1", v)
		}
		if shared {
			t.Error("first Do: shared should be false")
		}
	}()
	<-started

	// Forget the key while the call is in progress.
	sf.Forget("key")

	// A new Do should start a fresh execution.
	v, shared, err := sf.Do("key", func() (int, error) {
		execCount.Add(1)
		return 2, nil
	})
	if err != nil {
		t.Fatalf("new Do: %v", err)
	}
	if shared {
		t.Fatal("new Do: shared should be false")
	}
	if v != 2 {
		t.Fatalf("new Do: got %d, want 2", v)
	}

	close(unblock)
	firstDone.Wait()

	if got := execCount.Load(); got != 2 {
		t.Fatalf("fn executed %d times, want 2", got)
	}
}

func TestSingleFlight_Stress(t *testing.T) {
	sf := NewSingleFlight[int, int]()
	const keys = 5
	const perKey = 100
	var execCount atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})

	for k := range keys {
		for range perKey {
			wg.Add(1)
			go func(key int) {
				defer wg.Done()
				<-start
				v, _, err := sf.Do(key, func() (int, error) {
					execCount.Add(1)
					time.Sleep(10 * time.Millisecond)
					return key, nil
				})
				if err != nil {
					t.Errorf("key %d: %v", key, err)
				}
				if v != key {
					t.Errorf("key %d: got %d", key, v)
				}
			}(k)
		}
	}

	close(start)
	wg.Wait()

	if got := execCount.Load(); got != keys {
		t.Fatalf("fn executed %d times, want %d", got, keys)
	}
}

func TestSingleFlight_PanicRecovery(t *testing.T) {
	sf := NewSingleFlight[string, int]()

	const N = 10
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, N)

	for i := range N {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			_, _, errs[idx] = sf.Do("key", func() (int, error) {
				panic("boom")
			})
		}(i)
	}

	close(start)
	wg.Wait()

	for i, e := range errs {
		if e == nil {
			t.Fatalf("goroutine %d: expected error, got nil", i)
		}
		if !strings.Contains(e.Error(), "boom") {
			t.Errorf("goroutine %d: error %q should contain 'boom'", i, e.Error())
		}
	}
}

func TestSingleFlight_ZeroKeyAndValue(t *testing.T) {
	// Verify that zero-value keys (empty string, int 0) and zero-value results work correctly.
	t.Run("EmptyStringKey", func(t *testing.T) {
		sf := NewSingleFlight[string, int]()
		v, shared, err := sf.Do("", func() (int, error) {
			return 0, nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 0 {
			t.Fatalf("got %d, want 0 (zero value)", v)
		}
		if shared {
			t.Fatal("shared should be false")
		}
	})

	t.Run("IntZeroKey", func(t *testing.T) {
		sf := NewSingleFlight[int, string]()
		v, shared, err := sf.Do(0, func() (string, error) {
			return "", nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "" {
			t.Fatalf("got %q, want empty string (zero value)", v)
		}
		if shared {
			t.Fatal("shared should be false")
		}
	})
}

func TestSingleFlight_SequentialDo(t *testing.T) {
	sf := NewSingleFlight[string, int]()
	var execCount atomic.Int32

	fn := func() (int, error) {
		return int(execCount.Add(1)), nil
	}

	// Sequential Do calls on the same key without Forget.
	// Each call should execute fn independently after the previous completes.
	v1, shared1, err := sf.Do("key", fn)
	if err != nil {
		t.Fatalf("first Do: %v", err)
	}
	if v1 != 1 {
		t.Fatalf("first Do: got %d, want 1", v1)
	}
	if shared1 {
		t.Fatal("first Do: shared should be false")
	}

	v2, shared2, err := sf.Do("key", fn)
	if err != nil {
		t.Fatalf("second Do: %v", err)
	}
	if v2 != 2 {
		t.Fatalf("second Do: got %d, want 2", v2)
	}
	if shared2 {
		t.Fatal("second Do: shared should be false")
	}

	if got := execCount.Load(); got != 2 {
		t.Fatalf("fn executed %d times, want 2", got)
	}
}

func TestSingleFlight_ForgetNeverSeenKey(*testing.T) {
	sf := NewSingleFlight[string, int]()

	// Forget on a key that was never used in Do should not panic.
	sf.Forget("nonexistent")
	sf.Forget("")
}
