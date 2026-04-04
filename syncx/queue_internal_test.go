package syncx

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// --- ringBuf unit tests ---

func TestRingBuf_PushPopOrder(t *testing.T) {
	b := newRingBuf[int](3)
	b.push(1)
	b.push(2)
	b.push(3)

	if v := b.pop(); v != 1 {
		t.Fatalf("first pop: got %d, want 1", v)
	}
	if v := b.pop(); v != 2 {
		t.Fatalf("second pop: got %d, want 2", v)
	}
	if v := b.pop(); v != 3 {
		t.Fatalf("third pop: got %d, want 3", v)
	}
}

func TestRingBuf_FullAndEmpty(t *testing.T) {
	b := newRingBuf[int](2)

	if !b.empty() {
		t.Fatal("new buffer should be empty")
	}
	if b.full() {
		t.Fatal("new buffer should not be full")
	}

	b.push(1)
	if b.empty() {
		t.Fatal("after one push, should not be empty")
	}
	if b.full() {
		t.Fatal("one item in capacity-2, should not be full")
	}

	b.push(2)
	if !b.full() {
		t.Fatal("two items in capacity-2, should be full")
	}

	b.pop()
	if b.full() {
		t.Fatal("after one pop from full, should not be full")
	}
	if b.empty() {
		t.Fatal("after one pop, should not be empty")
	}

	b.pop()
	if !b.empty() {
		t.Fatal("after all pops, should be empty")
	}
}

func TestRingBuf_Peek(t *testing.T) {
	b := newRingBuf[int](3)

	if _, ok := b.peek(); ok {
		t.Fatal("peek on empty should return false")
	}

	b.push(42)
	v, ok := b.peek()
	if !ok || v != 42 {
		t.Fatalf("peek: got %d, %v; want 42, true", v, ok)
	}
	if b.len() != 1 {
		t.Fatal("peek should not remove item")
	}
}

func TestRingBuf_Len(t *testing.T) {
	b := newRingBuf[int](5)
	if b.len() != 0 {
		t.Fatalf("initial len: got %d, want 0", b.len())
	}
	b.push(1)
	b.push(2)
	if b.len() != 2 {
		t.Fatalf("after 2 pushes: got %d, want 2", b.len())
	}
	b.pop()
	if b.len() != 1 {
		t.Fatalf("after 1 pop: got %d, want 1", b.len())
	}
}

func TestRingBuf_CapacityOneWrapAround(t *testing.T) {
	b := newRingBuf[int](1)

	for i := range 10 {
		b.push(i)
		if !b.full() {
			t.Fatalf("round %d: after push, should be full", i)
		}
		v := b.pop()
		if v != i {
			t.Fatalf("round %d: got %d, want %d", i, v, i)
		}
		if !b.empty() {
			t.Fatalf("round %d: after pop, should be empty", i)
		}
	}
}

func TestRingBuf_CapacityTwoWrapAround(t *testing.T) {
	b := newRingBuf[int](2)

	// Fill and drain to advance head/tail past index 0
	b.push(1)
	b.push(2)
	b.pop() // head: 0→1
	b.pop() // head: 1→0 (wrap)

	if !b.empty() {
		t.Fatal("should be empty after drain")
	}

	// Push again to verify indices wrapped correctly
	b.push(3)
	b.push(4)
	if v := b.pop(); v != 3 {
		t.Fatalf("after wrap: got %d, want 3", v)
	}
	if v := b.pop(); v != 4 {
		t.Fatalf("after wrap: got %d, want 4", v)
	}
}

func TestRingBuf_PopZeroClearPointer(t *testing.T) {
	// Verify that pop clears the slot to help GC with pointer types.
	b := newRingBuf[*int](2)
	val := 42
	b.push(&val)
	b.pop()

	if b.buf[b.head] != nil {
		t.Fatal("pop should clear slot to nil for pointer types")
	}
}

// --- waitCond test ---

func TestWaitCond_ContextCancel(t *testing.T) {
	mu := sync.Mutex{}
	cond := sync.NewCond(&mu)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	mu.Lock()
	err := waitCond(ctx, cond)
	mu.Unlock()

	if err == nil {
		t.Fatal("waitCond should return error on context cancel")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want DeadlineExceeded", err)
	}
}

// --- BlockingQueue additional boundary tests ---

func TestBlockingQueue_CapacityTwoWrapAround(t *testing.T) {
	q := NewBlockingQueue[int](2)
	ctx := context.Background()

	// Round 1: fill and drain
	if err := q.Push(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := q.Push(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if v, err := q.Pop(ctx); err != nil || v != 1 {
		t.Fatalf("pop 1: got %d, %v", v, err)
	}
	if v, err := q.Pop(ctx); err != nil || v != 2 {
		t.Fatalf("pop 2: got %d, %v", v, err)
	}

	// Round 2: verify wrap-around correctness
	if err := q.Push(ctx, 3); err != nil {
		t.Fatal(err)
	}
	if err := q.Push(ctx, 4); err != nil {
		t.Fatal(err)
	}
	if v, err := q.Pop(ctx); err != nil || v != 3 {
		t.Fatalf("pop 3: got %d, %v", v, err)
	}
	if v, err := q.Pop(ctx); err != nil || v != 4 {
		t.Fatalf("pop 4: got %d, %v", v, err)
	}
}

func TestBlockingQueue_PointerGenericType(t *testing.T) {
	q := NewBlockingQueue[*string](3)
	ctx := context.Background()

	s1 := "hello"
	s2 := "world"

	if err := q.Push(ctx, &s1); err != nil {
		t.Fatal(err)
	}
	if err := q.Push(ctx, &s2); err != nil {
		t.Fatal(err)
	}

	v, err := q.Pop(ctx)
	if err != nil || *v != "hello" {
		t.Fatalf("pop 1: got %q, %v", deref(v), err)
	}

	v, err = q.Pop(ctx)
	if err != nil || *v != "world" {
		t.Fatalf("pop 2: got %q, %v", deref(v), err)
	}

	// Verify slots cleared after pop (GC assist for pointer types)
	for i := range q.ring.buf {
		if q.ring.buf[i] != nil {
			t.Fatalf("slot %d not nil after pop, potential GC leak", i)
		}
	}
}

// --- RingQueue additional boundary tests ---

func TestRingQueue_PointerGenericType(t *testing.T) {
	q := NewRingQueue[*string](2)

	s1 := "hello"
	s2 := "world"
	s3 := "overflow"

	q.Push(&s1)
	q.Push(&s2)
	q.Push(&s3) // evicts &s1

	v, err := q.Pop(context.Background())
	if err != nil || *v != "world" {
		t.Fatalf("pop 1: got %q, %v", deref(v), err)
	}

	v, err = q.Pop(context.Background())
	if err != nil || *v != "overflow" {
		t.Fatalf("pop 2: got %q, %v", deref(v), err)
	}

	// Verify evicted slot cleared
	for i := range q.ring.buf {
		if q.ring.buf[i] != nil {
			t.Fatalf("slot %d not nil after drain, potential GC leak", i)
		}
	}
}

func deref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}
