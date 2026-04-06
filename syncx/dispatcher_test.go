package syncx

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDispatcher_BasicSubmit(t *testing.T) {
	var count atomic.Int32
	d := NewDispatcher(1, func(_ string, _ int) error {
		count.Add(1)
		return nil
	}, WithBuffer[string, int](10))
	defer d.Close()

	if err := d.Submit("a", 1); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := d.Submit("b", 2); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	d.Close()

	if got := count.Load(); got != 2 {
		t.Fatalf("processed %d tasks, want 2", got)
	}
}

func TestDispatcher_SameKeySerial(t *testing.T) {
	var concurrent atomic.Int32

	d := NewDispatcher(4, func(_ string, _ int) error {
		if c := concurrent.Add(1); c > 1 {
			t.Errorf("same key processed concurrently: %d", c)
		}
		time.Sleep(time.Millisecond)
		concurrent.Add(-1)
		return nil
	}, WithBuffer[string, int](64))

	for i := range 50 {
		if err := d.Submit("key", i); err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}
	d.Close()
}

func TestDispatcher_DifferentKeysParallel(t *testing.T) {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	d := NewDispatcher(4, func(_ int, _ int) error {
		c := concurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if c <= old || maxConcurrent.CompareAndSwap(old, c) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		concurrent.Add(-1)
		return nil
	}, WithBuffer[int, int](64))

	for i := range 100 {
		if err := d.Submit(i, i); err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}
	d.Close()

	if maxConcurrent.Load() < 2 {
		t.Errorf("expected concurrent processing across slots, max concurrent = %d", maxConcurrent.Load())
	}
}

func TestDispatcher_SubmitBlocksWhenFull(t *testing.T) {
	unblock := make(chan struct{})

	d := NewDispatcher(1, func(_ string, _ int) error {
		<-unblock
		return nil
	}, WithBuffer[string, int](1))

	// First submit: slot goroutine pops immediately, handler blocks on unblock.
	if err := d.Submit("a", 1); err != nil {
		t.Fatalf("Submit a: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // let handler start and block

	// Second submit: fills the buffer (capacity 1).
	if err := d.Submit("b", 2); err != nil {
		t.Fatalf("Submit b: %v", err)
	}

	// Third submit: should block because handler is busy and buffer is full.
	done := make(chan struct{})
	go func() {
		if err := d.Submit("c", 3); err != nil && !errors.Is(err, ErrDispatcherClosed) {
			t.Errorf("Submit c: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Submit should have blocked")
	case <-time.After(50 * time.Millisecond):
		// expected: still blocked
	}

	close(unblock)
	<-done
	d.Close()
}

func TestDispatcher_TrySubmitFull(t *testing.T) {
	unblock := make(chan struct{})

	d := NewDispatcher(1, func(_ string, _ int) error {
		<-unblock
		return nil
	}, WithBuffer[string, int](1))

	if err := d.Submit("a", 1); err != nil {
		t.Fatalf("Submit a: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // let handler start and block
	if err := d.Submit("b", 2); err != nil {
		t.Fatalf("Submit b: %v", err)
	}

	if d.TrySubmit("c", 3) {
		t.Fatal("TrySubmit should return false when full")
	}

	close(unblock)
	d.Close()
}

func TestDispatcher_CloseWaitsPending(t *testing.T) {
	var processed atomic.Int32
	started := make(chan struct{})
	var once sync.Once

	d := NewDispatcher(1, func(_ string, _ int) error {
		once.Do(func() { close(started) })
		time.Sleep(10 * time.Millisecond)
		processed.Add(1)
		return nil
	}, WithBuffer[string, int](10))

	for i := range 5 {
		if err := d.Submit("key", i); err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}
	<-started

	d.Close()

	if got := processed.Load(); got != 5 {
		t.Fatalf("processed %d tasks, want 5", got)
	}
}

func TestDispatcher_SubmitAfterClose(t *testing.T) {
	d := NewDispatcher(1, func(_ string, _ int) error { return nil })
	d.Close()

	if err := d.Submit("a", 1); !errors.Is(err, ErrDispatcherClosed) {
		t.Fatalf("expected ErrDispatcherClosed, got %v", err)
	}
	if d.TrySubmit("a", 1) {
		t.Fatal("TrySubmit should return false after close")
	}
}

func TestDispatcher_OnError(t *testing.T) {
	testErr := errors.New("handler error")

	var gotKey string
	var gotVal int
	var gotErr error

	d := NewDispatcher(1, func(_ string, v int) error {
		if v == 42 {
			return testErr
		}
		return nil
	}, WithOnError(func(k string, v int, err error) {
		gotKey = k
		gotVal = v
		gotErr = err
		gotVal = v
		gotErr = err
	}), WithBuffer[string, int](10))

	if err := d.Submit("good", 1); err != nil {
		t.Fatalf("Submit good: %v", err)
	}
	if err := d.Submit("bad", 42); err != nil {
		t.Fatalf("Submit bad: %v", err)
	}
	d.Close()

	if gotKey != "bad" {
		t.Errorf("OnError key: got %q, want %q", gotKey, "bad")
	}
	if gotVal != 42 {
		t.Errorf("OnError value: got %d, want 42", gotVal)
	}
	if gotErr != testErr {
		t.Errorf("OnError err: got %v, want %v", gotErr, testErr)
	}
}

func TestDispatcher_ConcurrentStress(t *testing.T) {
	const workers = 4
	const tasksPerWorker = 100

	var count atomic.Int32

	d := NewDispatcher(8, func(_ int, _ int) error {
		count.Add(1)
		return nil
	}, WithBuffer[int, int](16))

	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for i := range tasksPerWorker {
				if err := d.Submit(offset*tasksPerWorker+i, i); err != nil {
					t.Errorf("Submit: %v", err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	d.Close()

	if got := count.Load(); got != workers*tasksPerWorker {
		t.Fatalf("processed %d tasks, want %d", got, workers*tasksPerWorker)
	}
}

func TestDispatcher_GracefulShutdown(t *testing.T) {
	const N = 100
	var processed atomic.Int32

	d := NewDispatcher(4, func(_ int, _ int) error {
		processed.Add(1)
		return nil
	}, WithBuffer[int, int](N))

	for i := range N {
		if err := d.Submit(i, i); err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
	}
	d.Close()

	if got := processed.Load(); got != N {
		t.Fatalf("processed %d tasks, want %d", got, N)
	}
}

func TestDispatcher_CloseIdempotent(t *testing.T) {
	d := NewDispatcher(2, func(_ string, _ int) error { return nil })
	if err := d.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestDispatcher_NewDispatcherPanics(t *testing.T) {
	t.Run("ZeroSlots", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for slots <= 0")
			}
		}()
		NewDispatcher(0, func(_ string, _ int) error { return nil })
	})

	t.Run("NilHandler", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for nil handler")
			}
		}()
		NewDispatcher[string, int](1, nil)
	})

	t.Run("NegativeBuffer", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for negative buffer")
			}
		}()
		NewDispatcher(1, func(_ string, _ int) error { return nil },
			WithBuffer[string, int](-1))
	})
}

func TestDispatcher_OnErrorContinuesProcessing(t *testing.T) {
	var count atomic.Int32

	d := NewDispatcher(1, func(_ string, v int) error {
		count.Add(1)
		if v == 2 {
			return errors.New("fail")
		}
		return nil
	}, WithOnError(func(_ string, _ int, _ error) {
		// error is logged, processing continues
	}), WithBuffer[string, int](10))

	if err := d.Submit("k", 1); err != nil {
		t.Fatalf("Submit k1: %v", err)
	}
	if err := d.Submit("k", 2); err != nil {
		t.Fatalf("Submit k2: %v", err)
	}
	if err := d.Submit("k", 3); err != nil {
		t.Fatalf("Submit k3: %v", err)
	}
	d.Close()

	if got := count.Load(); got != 3 {
		t.Fatalf("processed %d tasks, want 3 (handler error should not stop slot)", got)
	}
}

func TestDispatcher_DefaultConstruction(t *testing.T) {
	var count atomic.Int32

	// No options: default capacity 1 (handoff).
	d := NewDispatcher(1, func(_ string, _ int) error {
		count.Add(1)
		return nil
	})

	if err := d.Submit("a", 1); err != nil {
		t.Fatalf("Submit a: %v", err)
	}
	if err := d.Submit("b", 2); err != nil {
		t.Fatalf("Submit b: %v", err)
	}
	d.Close()

	if got := count.Load(); got != 2 {
		t.Fatalf("processed %d tasks, want 2", got)
	}
}

func TestDispatcher_WithBufferZero(t *testing.T) {
	var count atomic.Int32

	// WithBuffer(0) should behave like default (capacity 1 handoff).
	d := NewDispatcher(1, func(_ string, _ int) error {
		count.Add(1)
		return nil
	}, WithBuffer[string, int](0))

	if err := d.Submit("a", 1); err != nil {
		t.Fatalf("Submit a: %v", err)
	}
	if err := d.Submit("b", 2); err != nil {
		t.Fatalf("Submit b: %v", err)
	}
	d.Close()

	if got := count.Load(); got != 2 {
		t.Fatalf("processed %d tasks, want 2", got)
	}
}

func TestDispatcher_ConcurrentSubmitAndClose(t *testing.T) {
	const workers = 4
	const tasksPerWorker = 100
	var count atomic.Int32

	d := NewDispatcher(8, func(_ int, _ int) error {
		count.Add(1)
		return nil
	}, WithBuffer[int, int](16))

	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		if w == 0 {
			go func() {
				defer wg.Done()
				time.Sleep(time.Millisecond)
				d.Close()
			}()
			continue
		}
		go func(offset int) {
			defer wg.Done()
			for i := range tasksPerWorker {
				if err := d.Submit(offset*tasksPerWorker+i, i); err != nil {
					if !errors.Is(err, ErrDispatcherClosed) {
						t.Errorf("unexpected error: %v", err)
					}
					return
				}
			}
		}(w)
	}
	wg.Wait()

	// Some tasks must be processed; exact count depends on Close timing.
	if count.Load() == 0 {
		t.Fatal("expected at least one task to be processed")
	}
}

func TestDispatcher_MultiSlotOnError(t *testing.T) {
	var errorCount atomic.Int32

	d := NewDispatcher(4, func(_ int, v int) error {
		if v < 0 {
			return errors.New("negative")
		}
		return nil
	}, WithOnError(func(_ int, _ int, _ error) {
		errorCount.Add(1)
	}))

	for i := range 100 {
		if err := d.Submit(i, -1); err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
	}
	d.Close()

	if got := errorCount.Load(); got != 100 {
		t.Fatalf("OnError called %d times, want 100", got)
	}
}
