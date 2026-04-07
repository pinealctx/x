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

func TestBlockingQueue_PushPopOrder(t *testing.T) {
	q := syncx.NewBlockingQueue[int](10)
	for i := range 5 {
		if err := q.Push(context.Background(), i); err != nil {
			t.Fatalf("Push: %v", err)
		}
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

func TestBlockingQueue_FullBlocksPush(t *testing.T) {
	q := syncx.NewBlockingQueue[int](2)
	if err := q.Push(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := q.Push(context.Background(), 2); err != nil {
		t.Fatal(err)
	}

	pushed := make(chan struct{})
	go func() {
		q.Push(context.Background(), 3) //nolint:errcheck // result verified by subsequent Pop
		close(pushed)
	}()

	select {
	case <-pushed:
		t.Fatal("Push should have blocked")
	case <-time.After(50 * time.Millisecond):
	}

	v, err := q.Pop(context.Background())
	if err != nil || v != 1 {
		t.Fatalf("Pop: got %d, %v", v, err)
	}
	<-pushed

	// Remaining items: 2, 3
	v, err = q.Pop(context.Background())
	if err != nil || v != 2 {
		t.Fatalf("Pop: got %d, %v", v, err)
	}
	v, err = q.Pop(context.Background())
	if err != nil || v != 3 {
		t.Fatalf("Pop: got %d, %v", v, err)
	}
}

func TestBlockingQueue_EmptyBlocksPop(t *testing.T) {
	q := syncx.NewBlockingQueue[int](5)

	popped := make(chan int)
	go func() {
		v, _ := q.Pop(context.Background())
		popped <- v
	}()

	select {
	case <-popped:
		t.Fatal("Pop should have blocked")
	case <-time.After(50 * time.Millisecond):
	}

	if err := q.Push(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	select {
	case v := <-popped:
		if v != 42 {
			t.Fatalf("expected 42, got %d", v)
		}
	case <-time.After(time.Second):
		t.Fatal("Pop didn't unblock after Push")
	}
}

// --- Try operations ---

func TestBlockingQueue_TryPushFull(t *testing.T) {
	q := syncx.NewBlockingQueue[int](2)
	if err := q.TryPush(1); err != nil {
		t.Fatal(err)
	}
	if err := q.TryPush(2); err != nil {
		t.Fatal(err)
	}
	if err := q.TryPush(3); !errors.Is(err, syncx.ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

func TestBlockingQueue_TryPopEmpty(t *testing.T) {
	q := syncx.NewBlockingQueue[int](5)
	_, err := q.TryPop()
	if !errors.Is(err, syncx.ErrQueueEmpty) {
		t.Fatalf("expected ErrQueueEmpty, got %v", err)
	}
}

// --- Close (drain mode) ---

func TestBlockingQueue_CloseDrains(t *testing.T) {
	q := syncx.NewBlockingQueue[int](10)
	for _, v := range []int{1, 2, 3} {
		if err := q.Push(context.Background(), v); err != nil {
			t.Fatal(err)
		}
	}

	q.Close()

	err := q.Push(context.Background(), 4)
	if !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed, got %v", err)
	}

	for _, want := range []int{1, 2, 3} {
		v, err := q.Pop(context.Background())
		if err != nil {
			t.Fatalf("Pop: %v", err)
		}
		if v != want {
			t.Fatalf("expected %d, got %d", want, v)
		}
	}

	_, err = q.Pop(context.Background())
	if !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed after drain, got %v", err)
	}
}

// --- CloseNow (immediate stop) ---

func TestBlockingQueue_CloseNowImmediate(t *testing.T) {
	q := syncx.NewBlockingQueue[int](10)
	if err := q.Push(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := q.Push(context.Background(), 2); err != nil {
		t.Fatal(err)
	}

	q.CloseNow()

	err := q.Push(context.Background(), 3)
	if !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed, got %v", err)
	}

	_, err = q.Pop(context.Background())
	if !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed, got %v", err)
	}
}

// --- Idempotent close ---

func TestBlockingQueue_CloseIdempotent(_ *testing.T) {
	q := syncx.NewBlockingQueue[int](5)
	q.Close()
	q.Close()
	q.CloseNow()
}

func TestBlockingQueue_CloseNowIdempotent(_ *testing.T) {
	q := syncx.NewBlockingQueue[int](5)
	q.CloseNow()
	q.CloseNow()
	q.Close()
}

// --- Context cancellation ---

func TestBlockingQueue_PushCtxCancel(t *testing.T) {
	q := syncx.NewBlockingQueue[int](2)
	if err := q.Push(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := q.Push(context.Background(), 2); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := q.Push(ctx, 3)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestBlockingQueue_PopCtxCancel(t *testing.T) {
	q := syncx.NewBlockingQueue[int](5)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := q.Pop(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// --- Concurrent producer-consumer ---

func TestBlockingQueue_ConcurrentProdCons(t *testing.T) {
	const n = 1000
	q := syncx.NewBlockingQueue[int](64)
	var wg sync.WaitGroup
	var sum atomic.Int64

	wg.Go(func() {
		for i := 1; i <= n; i++ {
			if err := q.Push(context.Background(), i); err != nil {
				t.Errorf("Push: %v", err)
				return
			}
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
	expected := int64(n) * int64(n+1) / 2
	if sum.Load() != expected {
		t.Fatalf("expected sum %d, got %d", expected, sum.Load())
	}
}

func TestBlockingQueue_MultiProducerConsumer(t *testing.T) {
	const producers = 4
	const itemsPerProducer = 500
	q := syncx.NewBlockingQueue[int](32)

	var prodWg sync.WaitGroup
	for p := range producers {
		prodWg.Go(func() {
			for i := 1; i <= itemsPerProducer; i++ {
				q.Push(context.Background(), p*10000+i) //nolint:errcheck // test setup
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

	total := int64(producers * itemsPerProducer)
	if consumed.Load() != total {
		t.Fatalf("expected %d items, got %d", total, consumed.Load())
	}
}

// --- Len/Peek ---

func TestBlockingQueue_Len(t *testing.T) {
	q := syncx.NewBlockingQueue[int](10)
	if q.Len() != 0 {
		t.Fatalf("expected 0, got %d", q.Len())
	}
	if err := q.Push(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := q.Push(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if q.Len() != 2 {
		t.Fatalf("expected 2, got %d", q.Len())
	}
	if _, err := q.Pop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if q.Len() != 1 {
		t.Fatalf("expected 1, got %d", q.Len())
	}
}

func TestBlockingQueue_Peek(t *testing.T) {
	q := syncx.NewBlockingQueue[int](10)
	if _, ok := q.Peek(); ok {
		t.Fatal("Peek on empty should return false")
	}
	if err := q.Push(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	v, ok := q.Peek()
	if !ok || v != 42 {
		t.Fatalf("Peek: got %d, %v", v, ok)
	}
	if q.Len() != 1 {
		t.Fatal("Peek should not remove item")
	}
}

// --- Try after close ---

func TestBlockingQueue_TryPushAfterClose(t *testing.T) {
	q := syncx.NewBlockingQueue[int](5)
	q.Close()
	if err := q.TryPush(1); !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed, got %v", err)
	}
}

func TestBlockingQueue_TryPopAfterCloseDrain(t *testing.T) {
	q := syncx.NewBlockingQueue[int](5)
	if err := q.Push(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	q.Close()

	v, err := q.TryPop()
	if err != nil || v != 1 {
		t.Fatalf("TryPop: got %d, %v", v, err)
	}

	_, err = q.TryPop()
	if !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed, got %v", err)
	}
}

func TestBlockingQueue_TryPopAfterCloseNow(t *testing.T) {
	q := syncx.NewBlockingQueue[int](5)
	if err := q.Push(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	q.CloseNow()

	_, err := q.TryPop()
	if !errors.Is(err, syncx.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed, got %v", err)
	}
}

// --- Capacity validation ---

func TestBlockingQueue_PanicOnZeroCapacity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for zero capacity")
		}
	}()
	syncx.NewBlockingQueue[int](0)
}

func TestBlockingQueue_PanicOnNegativeCapacity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for negative capacity")
		}
	}()
	syncx.NewBlockingQueue[int](-1)
}

// --- Pop/Push unblocks on Close ---

func TestBlockingQueue_PopUnblocksOnClose(t *testing.T) {
	q := syncx.NewBlockingQueue[int](5)

	popDone := make(chan error, 1)
	go func() {
		_, err := q.Pop(context.Background())
		popDone <- err
	}()

	runtime.Gosched()
	q.Close()

	select {
	case err := <-popDone:
		if !errors.Is(err, syncx.ErrQueueClosed) {
			t.Fatalf("expected ErrQueueClosed, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Pop didn't unblock on Close")
	}
}

func TestBlockingQueue_PopUnblocksOnCloseNow(t *testing.T) {
	q := syncx.NewBlockingQueue[int](5)

	popDone := make(chan error, 1)
	go func() {
		_, err := q.Pop(context.Background())
		popDone <- err
	}()

	runtime.Gosched()
	q.CloseNow()

	select {
	case err := <-popDone:
		if !errors.Is(err, syncx.ErrQueueClosed) {
			t.Fatalf("expected ErrQueueClosed, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Pop didn't unblock on CloseNow")
	}
}

func TestBlockingQueue_PushUnblocksOnClose(t *testing.T) {
	q := syncx.NewBlockingQueue[int](1)
	if err := q.Push(context.Background(), 1); err != nil {
		t.Fatal(err)
	}

	pushDone := make(chan error, 1)
	go func() {
		pushDone <- q.Push(context.Background(), 2)
	}()

	runtime.Gosched()
	q.Close()

	select {
	case err := <-pushDone:
		if !errors.Is(err, syncx.ErrQueueClosed) {
			t.Fatalf("expected ErrQueueClosed, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Push didn't unblock on Close")
	}
}

// --- Capacity 1 ring buffer wrap-around ---

func TestBlockingQueue_CapacityOneWrapAround(t *testing.T) {
	q := syncx.NewBlockingQueue[int](1)
	for i := range 10 {
		if err := q.Push(context.Background(), i); err != nil {
			t.Fatalf("Push %d: %v", i, err)
		}
		v, err := q.Pop(context.Background())
		if err != nil {
			t.Fatalf("Pop %d: %v", i, err)
		}
		if v != i {
			t.Fatalf("round %d: expected %d, got %d", i, i, v)
		}
	}
}

// --- No data race ---

func TestBlockingQueue_PushContextCancelWhileFull(t *testing.T) {
	q := syncx.NewBlockingQueue[int](1)
	ctx := context.Background()

	// Fill the queue.
	if err := q.Push(ctx, 1); err != nil {
		t.Fatal(err)
	}

	// Cancel context before Push (queue full, should return context error).
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // cancel immediately

	err := q.Push(cancelCtx, 2)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestBlockingQueue_PopContextCancelWhileEmpty(t *testing.T) {
	q := syncx.NewBlockingQueue[int](1)

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := q.Pop(cancelCtx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestBlockingQueue_NoDataRace(_ *testing.T) {
	q := syncx.NewBlockingQueue[int](64)
	var wg sync.WaitGroup

	for range 100 {
		wg.Go(func() {
			q.Push(context.Background(), 1) //nolint:errcheck // discard is intentional
		})
	}
	for range 100 {
		wg.Go(func() {
			q.Pop(context.Background()) //nolint:errcheck // discard is intentional

		})
	}

	wg.Wait()
}
