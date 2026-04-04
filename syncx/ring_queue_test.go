package syncx_test

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pinealctx/x/syncx"
)

// --- Basic operations ---

func TestRingQueue_PushPopOrder(t *testing.T) {
	q := syncx.NewRingQueue[int](5)
	for i := range 5 {
		q.Push(i)
	}
	for i := range 5 {
		v, err := q.Pop(context.Background())
		if err != nil {
			t.Fatalf("Pop: %v", err)
		}
		if v != i {
			t.Fatalf("expected %d, got %d", i, v)
		}
	}
}

// --- Eviction ---

func TestRingQueue_PushEvictsOldest(t *testing.T) {
	q := syncx.NewRingQueue[int](3)
	q.Push(1)
	q.Push(2)
	q.Push(3)
	// Queue full: [1, 2, 3], Push discards 1.
	q.Push(4)

	want := []int{2, 3, 4}
	for _, w := range want {
		v, err := q.Pop(context.Background())
		if err != nil {
			t.Fatalf("Pop: %v", err)
		}
		if v != w {
			t.Fatalf("expected %d, got %d", w, v)
		}
	}
}

func TestRingQueue_PushEvictReturnsEvicted(t *testing.T) {
	q := syncx.NewRingQueue[int](3)
	q.Push(1)
	q.Push(2)

	// Not full yet (2/3): no eviction.
	_, ok := q.PushEvict(3)
	if ok {
		t.Fatal("should not have evicted — queue was not full")
	}

	// Now full (3/3): eviction happens.
	evicted, ok := q.PushEvict(4)
	if !ok {
		t.Fatal("expected eviction")
	}
	if evicted != 1 {
		t.Fatalf("expected evicted=1, got %d", evicted)
	}
}

// --- Blocking Pop ---

func TestRingQueue_PopBlocksWhenEmpty(t *testing.T) {
	q := syncx.NewRingQueue[int](5)

	popped := make(chan int, 1)
	go func() {
		v, _ := q.Pop(context.Background())
		popped <- v
	}()

	runtime.Gosched()
	q.Push(42)

	select {
	case v := <-popped:
		if v != 42 {
			t.Fatalf("expected 42, got %d", v)
		}
	case <-time.After(time.Second):
		t.Fatal("Pop didn't unblock after Push")
	}
}

// --- TryPop ---

func TestRingQueue_TryPopEmpty(t *testing.T) {
	q := syncx.NewRingQueue[int](5)
	_, err := q.TryPop()
	if !errors.Is(err, syncx.ErrQueueEmpty) {
		t.Fatalf("expected ErrQueueEmpty, got %v", err)
	}
}

func TestRingQueue_TryPopSuccess(t *testing.T) {
	q := syncx.NewRingQueue[int](5)
	q.Push(99)
	v, err := q.TryPop()
	if err != nil || v != 99 {
		t.Fatalf("got %d, %v", v, err)
	}
}

// --- Close drain ---

func TestRingQueue_CloseDrains(t *testing.T) {
	q := syncx.NewRingQueue[int](10)
	q.Push(1)
	q.Push(2)
	q.Push(3)
	q.Close()

	// Push after close is silently discarded.
	q.Push(4)

	for _, want := range []int{1, 2, 3} {
		v, err := q.Pop(context.Background())
		if err != nil {
			t.Fatalf("Pop: %v", err)
		}
		if v != want {
			t.Fatalf("expected %d, got %d", want, v)
		}
	}

	_, err := q.Pop(context.Background())
	if !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed after drain, got %v", err)
	}
}

// --- CloseNow ---

func TestRingQueue_CloseNowImmediate(t *testing.T) {
	q := syncx.NewRingQueue[int](10)
	q.Push(1)
	q.Push(2)
	q.CloseNow()

	_, err := q.Pop(context.Background())
	if !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed, got %v", err)
	}
}

// --- Idempotent close ---

func TestRingQueue_CloseIdempotent(t *testing.T) {
	tests := []struct {
		name  string
		first func(q *syncx.RingQueue[int])
		rest  func(q *syncx.RingQueue[int])
	}{
		{"Close_then_Close_and_CloseNow",
			func(q *syncx.RingQueue[int]) { q.Close() },
			func(q *syncx.RingQueue[int]) { q.Close(); q.CloseNow() },
		},
		{"CloseNow_then_CloseNow_and_Close",
			func(q *syncx.RingQueue[int]) { q.CloseNow() },
			func(q *syncx.RingQueue[int]) { q.CloseNow(); q.Close() },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			q := syncx.NewRingQueue[int](5)
			tt.first(q)
			tt.rest(q)
		})
	}
}

// --- Context cancellation ---

func TestRingQueue_PopCtxCancel(t *testing.T) {
	q := syncx.NewRingQueue[int](5)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := q.Pop(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// --- Concurrent ---

func TestRingQueue_ConcurrentProdCons(t *testing.T) {
	const n = 2000
	q := syncx.NewRingQueue[int](64)
	var wg sync.WaitGroup
	var sum atomic.Int64

	wg.Go(func() {
		for i := 1; i <= n; i++ {
			q.Push(i)
		}
		q.Close()
	})

	wg.Go(func() {
		for {
			v, err := q.Pop(context.Background())
			if errors.Is(err, syncx.ErrQueueClosed) {
				return
			}
			if err != nil {
				t.Errorf("Pop: %v", err)
				return
			}
			sum.Add(int64(v))
		}
	})

	wg.Wait()
	// Note: sum may be < expected because Push discards oldest when full.
	// We just verify no data race and all received items are valid.
}

func TestRingQueue_MultiProducerConsumer(t *testing.T) {
	const producers = 4
	const itemsPerProducer = 500
	q := syncx.NewRingQueue[int](32)

	var prodWg sync.WaitGroup
	for p := range producers {
		prodWg.Go(func() {
			for i := 1; i <= itemsPerProducer; i++ {
				q.Push(p*10000 + i) //nolint:errcheck // fire-and-forget
			}
		})
	}

	go func() {
		prodWg.Wait()
		q.Close()
	}()

	var consumed atomic.Int64
	var consWg sync.WaitGroup
	for range 2 {
		consWg.Go(func() {
			for {
				_, err := q.Pop(context.Background())
				if errors.Is(err, syncx.ErrQueueClosed) {
					return
				}
				if err != nil {
					t.Errorf("Pop: %v", err)
					return
				}
				consumed.Add(1)
			}
		})
	}
	consWg.Wait()

	// consumed may be < total because of eviction, must be > 0.
	if consumed.Load() == 0 {
		t.Fatal("expected at least some items to be consumed")
	}
}

// --- Len/Peek ---

func TestRingQueue_Len(t *testing.T) {
	q := syncx.NewRingQueue[int](10)
	if q.Len() != 0 {
		t.Fatalf("expected 0, got %d", q.Len())
	}
	q.Push(1)
	q.Push(2)
	if q.Len() != 2 {
		t.Fatalf("expected 2, got %d", q.Len())
	}
	if _, err := q.TryPop(); err != nil {
		t.Fatal(err)
	}
	if q.Len() != 1 {
		t.Fatalf("expected 1, got %d", q.Len())
	}
}

func TestRingQueue_Peek(t *testing.T) {
	q := syncx.NewRingQueue[int](10)
	if _, ok := q.Peek(); ok {
		t.Fatal("Peek on empty should return false")
	}
	q.Push(42)
	v, ok := q.Peek()
	if !ok || v != 42 {
		t.Fatalf("Peek: got %d, %v", v, ok)
	}
	if q.Len() != 1 {
		t.Fatal("Peek should not remove item")
	}
}

// --- Close/CloseNow unblocks Pop ---

func TestRingQueue_PopUnblocksOnClose(t *testing.T) {
	q := syncx.NewRingQueue[int](5)

	done := make(chan error, 1)
	go func() {
		_, err := q.Pop(context.Background())
		done <- err
	}()

	runtime.Gosched()
	q.Close()

	select {
	case err := <-done:
		if !errors.Is(err, syncx.ErrQueueClosed) {
			t.Fatalf("expected ErrQueueClosed, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Pop didn't unblock on Close")
	}
}

func TestRingQueue_PopUnblocksOnCloseNow(t *testing.T) {
	q := syncx.NewRingQueue[int](5)

	done := make(chan error, 1)
	go func() {
		_, err := q.Pop(context.Background())
		done <- err
	}()

	runtime.Gosched()
	q.CloseNow()

	select {
	case err := <-done:
		if !errors.Is(err, syncx.ErrQueueClosed) {
			t.Fatalf("expected ErrQueueClosed, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Pop didn't unblock on CloseNow")
	}
}

// --- Push after close ---

func TestRingQueue_PushAfterClose(t *testing.T) {
	q := syncx.NewRingQueue[int](5)
	q.Push(1)
	q.Close()
	q.Push(2) // silently discarded

	v, err := q.Pop(context.Background())
	if err != nil || v != 1 {
		t.Fatalf("expected 1, got %d, %v", v, err)
	}

	_, err = q.Pop(context.Background())
	if !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed, got %v", err)
	}
}

// --- Capacity validation ---

func TestRingQueue_PanicOnZeroCapacity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for zero capacity")
		}
	}()
	syncx.NewRingQueue[int](0)
}

func TestRingQueue_PanicOnNegativeCapacity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for negative capacity")
		}
	}()
	syncx.NewRingQueue[int](-1)
}

// --- Wrap-around ---

func TestRingQueue_CapacityOneWrapAround(t *testing.T) {
	q := syncx.NewRingQueue[int](1)
	for i := range 10 {
		q.Push(i)
		v, err := q.Pop(context.Background())
		if err != nil {
			t.Fatalf("Pop %d: %v", i, err)
		}
		if v != i {
			t.Fatalf("expected %d, got %d", i, v)
		}
	}

	// Now test eviction with PushEvict on capacity-1: full → evict.
	q.Push(100)
	evicted, ok := q.PushEvict(200)
	if !ok {
		t.Fatal("expected eviction on full capacity-1 queue")
	}
	if evicted != 100 {
		t.Fatalf("expected evicted=100, got %d", evicted)
	}
	v, err := q.Pop(context.Background())
	if err != nil || v != 200 {
		t.Fatalf("expected 200, got %d, %v", v, err)
	}
}

// --- Capacity-2 wrap-around ---

func TestRingQueue_CapacityTwoWrapAround(t *testing.T) {
	q := syncx.NewRingQueue[int](2)

	// Fill: [0, 1]
	q.Push(0)
	q.Push(1)

	// Drain: empty
	for _, want := range []int{0, 1} {
		v, err := q.Pop(context.Background())
		if err != nil || v != want {
			t.Fatalf("expected %d, got %d, %v", want, v, err)
		}
	}

	// Refill after wrap: head==tail==0, now [2, 3]
	q.Push(2)
	q.Push(3)

	// Drain again — head/tail wrapped around the ring.
	for _, want := range []int{2, 3} {
		v, err := q.Pop(context.Background())
		if err != nil || v != want {
			t.Fatalf("expected %d, got %d, %v", want, v, err)
		}
	}
}

// --- TryPop with closedDrain ---

func TestRingQueue_TryPopCloseDrain(t *testing.T) {
	q := syncx.NewRingQueue[int](5)
	q.Push(10)
	q.Push(20)
	q.Close() // closedDrain: remaining items can still be consumed.

	// TryPop should return items while the queue is non-empty.
	v, err := q.TryPop()
	if err != nil || v != 10 {
		t.Fatalf("first TryPop: got %d, %v", v, err)
	}

	v, err = q.TryPop()
	if err != nil || v != 20 {
		t.Fatalf("second TryPop: got %d, %v", v, err)
	}

	// Queue now empty + closedDrain → ErrQueueClosed.
	_, err = q.TryPop()
	if !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed after drain, got %v", err)
	}
}

// --- No data race ---

func TestRingQueue_NoDataRace(_ *testing.T) {
	q := syncx.NewRingQueue[int](128)
	var wg sync.WaitGroup

	// Push 100 items first so Pop won't block.
	for i := range 100 {
		q.Push(i)
	}

	for range 100 {
		wg.Go(func() {
			q.Push(1) //nolint:errcheck // discard is intentional
		})
	}
	for range 100 {
		wg.Go(func() {
			q.Pop(context.Background()) //nolint:errcheck // discard is intentional
		})
	}

	wg.Wait()
	q.Close()
}
