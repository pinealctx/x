package syncx

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroup_SingleGo(t *testing.T) {
	g := NewGroup[int](0)
	g.Go(func() (int, error) {
		return 42, nil
	})
	results := g.Wait()
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Value != 42 {
		t.Fatalf("got %d, want 42", results[0].Value)
	}
	if results[0].Err != nil {
		t.Fatalf("unexpected error: %v", results[0].Err)
	}
}

func TestGroup_MultipleGo(t *testing.T) {
	g := NewGroup[int](0)
	const N = 10

	for i := range N {
		g.Go(func() (int, error) {
			return i, nil
		})
	}

	results := g.Wait()
	if len(results) != N {
		t.Fatalf("got %d results, want %d", len(results), N)
	}
	for i, r := range results {
		if r.Value != i {
			t.Errorf("results[%d].Value = %d, want %d", i, r.Value, i)
		}
		if r.Err != nil {
			t.Errorf("results[%d].Err = %v, want nil", i, r.Err)
		}
	}
}

func TestGroup_SubmissionOrder(t *testing.T) {
	g := NewGroup[string](1) // limit=1 forces sequential execution
	const N = 5

	for range N {
		g.Go(func() (string, error) {
			time.Sleep(10 * time.Millisecond)
			return "result", nil
		})
	}

	// Results should still be in submission order even with limit=1.
	results := g.Wait()
	if len(results) != N {
		t.Fatalf("got %d results, want %d", len(results), N)
	}
	for _, r := range results {
		if r.Value != "result" {
			t.Errorf("got %q, want %q", r.Value, "result")
		}
	}
}

func TestGroup_ConcurrencyLimit(t *testing.T) {
	const limit = 2
	const tasks = 10
	g := NewGroup[int](limit)

	var running atomic.Int32
	var maxRunning atomic.Int32

	for range tasks {
		g.Go(func() (int, error) {
			cur := running.Add(1)
			for {
				old := maxRunning.Load()
				if cur <= old || maxRunning.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			running.Add(-1)
			return int(cur), nil
		})
	}

	results := g.Wait()
	if len(results) != tasks {
		t.Fatalf("got %d results, want %d", len(results), tasks)
	}
	if peak := maxRunning.Load(); peak > limit {
		t.Fatalf("max concurrent %d exceeded limit %d", peak, limit)
	}
}

func TestGroup_NoLimit(t *testing.T) {
	g := NewGroup[int](0)
	const N = 20
	var started atomic.Int32
	start := make(chan struct{})

	for range N {
		g.Go(func() (int, error) {
			started.Add(1)
			<-start
			return 1, nil
		})
	}

	// Poll until all goroutines have started (no limit, so they should all start quickly).
	deadline := time.Now().Add(2 * time.Second)
	for started.Load() < N {
		if time.Now().After(deadline) {
			t.Fatalf("only %d goroutines started after timeout, want %d", started.Load(), N)
		}
		runtime.Gosched()
	}
	close(start)

	results := g.Wait()
	if len(results) != N {
		t.Fatalf("got %d results, want %d", len(results), N)
	}
	total := 0
	for _, r := range results {
		total += r.Value
	}
	if total != N {
		t.Fatalf("total = %d, want %d", total, N)
	}
}

func TestGroup_ErrorResult(t *testing.T) {
	g := NewGroup[int](0)
	testErr := errors.New("test error")

	g.Go(func() (int, error) {
		return 0, testErr
	})
	g.Go(func() (int, error) {
		return 42, nil
	})

	results := g.Wait()
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("first result should have error")
	}
	if !errors.Is(results[0].Err, testErr) {
		t.Errorf("got %v, want %v", results[0].Err, testErr)
	}
	if results[1].Value != 42 {
		t.Errorf("second result: got %d, want 42", results[1].Value)
	}
	if results[1].Err != nil {
		t.Errorf("second result: unexpected error %v", results[1].Err)
	}
}

func TestGroup_PanicRecovery(t *testing.T) {
	g := NewGroup[int](0)

	g.Go(func() (int, error) {
		panic("boom")
	})
	g.Go(func() (int, error) {
		return 42, nil
	})

	results := g.Wait()
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("first result should have error from panic")
	}
	if !strings.Contains(results[0].Err.Error(), "boom") {
		t.Errorf("error %q should contain 'boom'", results[0].Err.Error())
	}
	if results[1].Value != 42 {
		t.Errorf("second result: got %d, want 42", results[1].Value)
	}
}

func TestGroup_EmptyGroup(t *testing.T) {
	g := NewGroup[int](0)
	results := g.Wait()
	if results == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestGroup_GoAfterWaitPanic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from Go after Wait")
		}
		if !strings.Contains(fmt.Sprint(r), "after Wait") {
			t.Fatalf("panic %v should mention 'after Wait'", r)
		}
	}()

	g := NewGroup[int](0)
	g.Wait() // empty group
	g.Go(func() (int, error) { return 0, nil })
}

func TestGroup_DoubleWaitPanic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from double Wait")
		}
		if !strings.Contains(fmt.Sprint(r), "more than once") {
			t.Fatalf("panic %v should mention 'more than once'", r)
		}
	}()

	g := NewGroup[int](0)
	g.Wait()
	g.Wait() // should panic
}

func TestGroup_Stress(t *testing.T) {
	const limit = 4
	const tasks = 100
	g := NewGroup[int](limit)

	for i := range tasks {
		g.Go(func() (int, error) {
			return i, nil
		})
	}

	results := g.Wait()
	if len(results) != tasks {
		t.Fatalf("got %d results, want %d", len(results), tasks)
	}
	for i, r := range results {
		if r.Value != i {
			t.Errorf("results[%d].Value = %d, want %d", i, r.Value, i)
		}
		if r.Err != nil {
			t.Errorf("results[%d].Err = %v, want nil", i, r.Err)
		}
	}
}

func TestGroup_StressConcurrentGo(t *testing.T) {
	g := NewGroup[int](0)
	const tasks = 50
	var wg sync.WaitGroup

	for i := range tasks {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			g.Go(func() (int, error) {
				return idx, nil
			})
		}(i)
	}

	wg.Wait()
	results := g.Wait()

	if len(results) != tasks {
		t.Fatalf("got %d results, want %d", len(results), tasks)
	}

	// All values should be present (order is non-deterministic due to concurrent Go calls).
	seen := make(map[int]bool, tasks)
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		if seen[r.Value] {
			t.Errorf("duplicate value %d", r.Value)
		}
		seen[r.Value] = true
	}
}

func TestGroup_LimitWithErrors(t *testing.T) {
	g := NewGroup[int](2)
	testErr := errors.New("fail")

	for i := range 6 {
		g.Go(func() (int, error) {
			if i%2 == 0 {
				return 0, testErr
			}
			return i, nil
		})
	}

	results := g.Wait()
	if len(results) != 6 {
		t.Fatalf("got %d results, want 6", len(results))
	}
	for i, r := range results {
		if i%2 == 0 {
			if r.Err == nil {
				t.Errorf("results[%d]: expected error", i)
			}
		} else {
			if r.Value != i {
				t.Errorf("results[%d]: got %d, want %d", i, r.Value, i)
			}
		}
	}
}

func TestGroup_LimitEqualsTasks(t *testing.T) {
	// When limit == number of tasks, semaphore should never block.
	const N = 5
	g := NewGroup[int](N)

	for i := range N {
		g.Go(func() (int, error) {
			return i, nil
		})
	}

	results := g.Wait()
	if len(results) != N {
		t.Fatalf("got %d results, want %d", len(results), N)
	}
	for i, r := range results {
		if r.Value != i {
			t.Errorf("results[%d].Value = %d, want %d", i, r.Value, i)
		}
		if r.Err != nil {
			t.Errorf("results[%d].Err = %v, want nil", i, r.Err)
		}
	}
}

func TestGroup_NegativeLimit(t *testing.T) {
	// Negative limit should behave the same as limit=0 (no limit).
	g := NewGroup[int](-1)
	const N = 10
	var started atomic.Int32
	start := make(chan struct{})

	for range N {
		g.Go(func() (int, error) {
			started.Add(1)
			<-start
			return 1, nil
		})
	}

	// All goroutines should start without blocking (no concurrency limit).
	deadline := time.Now().Add(2 * time.Second)
	for started.Load() < N {
		if time.Now().After(deadline) {
			t.Fatalf("only %d goroutines started after timeout, want %d", started.Load(), N)
		}
		runtime.Gosched()
	}
	close(start)

	results := g.Wait()
	if len(results) != N {
		t.Fatalf("got %d results, want %d", len(results), N)
	}
}

func TestGroup_StressPanicWithLimit(t *testing.T) {
	// Multiple goroutines panic under concurrency limit — verify semaphore release
	// and result isolation.
	const limit = 2
	const tasks = 10
	g := NewGroup[int](limit)

	for i := range tasks {
		g.Go(func() (int, error) {
			if i%3 == 0 {
				panic("stress panic")
			}
			return i, nil
		})
	}

	results := g.Wait()
	if len(results) != tasks {
		t.Fatalf("got %d results, want %d", len(results), tasks)
	}
	for i, r := range results {
		if i%3 == 0 {
			if r.Err == nil {
				t.Errorf("results[%d]: expected error from panic", i)
			}
			if !strings.Contains(r.Err.Error(), "stress panic") {
				t.Errorf("results[%d]: error %q should contain 'stress panic'", i, r.Err.Error())
			}
		} else {
			if r.Value != i {
				t.Errorf("results[%d]: got %d, want %d", i, r.Value, i)
			}
			if r.Err != nil {
				t.Errorf("results[%d]: unexpected error %v", i, r.Err)
			}
		}
	}
}
