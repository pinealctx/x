package ds

import (
	"cmp"
	"slices"
	"testing"
)

func TestHeap_MinHeapPushPop(t *testing.T) {
	h := NewMinHeap[int]()
	h.Push(3)
	h.Push(1)
	h.Push(4)
	h.Push(1)
	h.Push(5)

	var got []int
	for {
		v, ok := h.Pop()
		if !ok {
			break
		}
		got = append(got, v)
	}
	if !slices.Equal(got, []int{1, 1, 3, 4, 5}) {
		t.Fatalf("min-heap pop order = %v, want [1 1 3 4 5]", got)
	}
}

func TestHeap_MaxHeap(t *testing.T) {
	h := NewMaxHeap[int]()
	h.Push(3)
	h.Push(1)
	h.Push(4)
	h.Push(1)
	h.Push(5)

	var got []int
	for {
		v, ok := h.Pop()
		if !ok {
			break
		}
		got = append(got, v)
	}
	if !slices.Equal(got, []int{5, 4, 3, 1, 1}) {
		t.Fatalf("max-heap pop order = %v, want [5 4 3 1 1]", got)
	}
}

func TestHeap_CustomCompare(t *testing.T) {
	type task struct {
		name     string
		priority int
	}
	h := NewHeap(func(a, b task) int {
		return cmp.Compare(a.priority, b.priority)
	})
	h.Push(task{"low", 3})
	h.Push(task{"high", 1})
	h.Push(task{"mid", 2})

	v, ok := h.Pop()
	if !ok || v.name != "high" {
		t.Fatalf("first pop = %+v, want high", v)
	}
}

func TestHeap_NewHeapFrom(t *testing.T) {
	data := []int{9, 7, 5, 3, 1, 2, 4, 6, 8}
	h := NewHeapFrom(cmp.Compare[int], data)

	var got []int
	for {
		v, ok := h.Pop()
		if !ok {
			break
		}
		got = append(got, v)
	}
	want := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	if !slices.Equal(got, want) {
		t.Fatalf("heapify pop order = %v, want %v", got, want)
	}
}

func TestHeap_Peek(t *testing.T) {
	h := NewMinHeap[int]()
	_, ok := h.Peek()
	if ok {
		t.Fatal("Peek on empty should return false")
	}

	h.Push(5)
	h.Push(3)
	h.Push(7)

	v, ok := h.Peek()
	if !ok || v != 3 {
		t.Fatalf("Peek() = %d, want 3", v)
	}
	if h.Len() != 3 {
		t.Fatalf("Len after Peek = %d, want 3", h.Len())
	}
}

func TestHeap_EmptyOperations(t *testing.T) {
	h := NewMinHeap[int]()
	_, ok := h.Pop()
	if ok {
		t.Fatal("Pop on empty should return false")
	}
	_, ok = h.Peek()
	if ok {
		t.Fatal("Peek on empty should return false")
	}
	if h.Len() != 0 {
		t.Fatal("Len on empty should be 0")
	}
}

func TestHeap_Drain(t *testing.T) {
	h := NewMinHeap[int]()
	h.Push(5)
	h.Push(3)
	h.Push(1)
	h.Push(4)
	h.Push(2)

	var drained []int
	for v := range h.Drain() {
		drained = append(drained, v)
	}
	if !slices.Equal(drained, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("Drain = %v, want [1 2 3 4 5]", drained)
	}
	if h.Len() != 0 {
		t.Fatalf("Len after Drain = %d, want 0", h.Len())
	}
}

func TestHeap_AllDoesNotModify(t *testing.T) {
	h := NewMinHeap[int]()
	h.Push(3)
	h.Push(1)
	h.Push(2)

	collected := map[int]bool{}
	for v := range h.All() {
		collected[v] = true
	}
	if len(collected) != 3 {
		t.Fatalf("All yielded %d elements, want 3", len(collected))
	}
	if h.Len() != 3 {
		t.Fatalf("Len after All = %d, want 3", h.Len())
	}
}

func TestHeap_Clear(t *testing.T) {
	h := NewMinHeap[int]()
	h.Push(1)
	h.Push(2)
	h.Clear()
	if h.Len() != 0 {
		t.Fatalf("Len after Clear = %d, want 0", h.Len())
	}
	// can still use after clear
	h.Push(10)
	v, ok := h.Peek()
	if !ok || v != 10 {
		t.Fatalf("after Clear+Push: Peek = %d, want 10", v)
	}
}

func TestHeap_DrainBreak(t *testing.T) {
	h := NewMinHeap[int]()
	for v := range 10 {
		h.Push(v)
	}
	var got []int
	for v := range h.Drain() {
		got = append(got, v)
		if v == 2 {
			break
		}
	}
	if !slices.Equal(got, []int{0, 1, 2}) {
		t.Fatalf("Drain break = %v, want [0 1 2]", got)
	}
	// remaining elements are still accessible
	if h.Len() != 7 {
		t.Fatalf("remaining Len = %d, want 7", h.Len())
	}
}

func TestHeap_SingleElement(t *testing.T) {
	h := NewMinHeap[int]()
	h.Push(42)

	v, ok := h.Peek()
	if !ok || v != 42 {
		t.Fatalf("Peek = %d, want 42", v)
	}
	if h.Len() != 1 {
		t.Fatalf("Len = %d, want 1", h.Len())
	}

	v, ok = h.Pop()
	if !ok || v != 42 {
		t.Fatalf("Pop = %d, want 42", v)
	}
	if h.Len() != 0 {
		t.Fatalf("Len after pop = %d, want 0", h.Len())
	}

	// empty after single pop
	_, ok = h.Peek()
	if ok {
		t.Fatal("Peek after last pop should return false")
	}
}

func TestHeap_AllBreak(t *testing.T) {
	h := NewMinHeap[int]()
	h.Push(10)
	h.Push(20)
	h.Push(30)
	h.Push(40)

	var count int
	for range h.All() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Fatalf("All break count = %d, want 2", count)
	}
	// heap should be unmodified
	if h.Len() != 4 {
		t.Fatalf("Len after All break = %d, want 4", h.Len())
	}
}

func TestHeap_NilComparePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil compare")
		}
	}()
	NewHeap[int](nil)
}

func TestHeap_NilComparePanics_NewHeapFrom(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil compare in NewHeapFrom")
		}
	}()
	NewHeapFrom(nil, []int{1, 2, 3})
}

func TestHeap_ConcurrentRead(t *testing.T) {
	h := NewMinHeap[int]()
	for i := range 100 {
		h.Push(i)
	}
	done := make(chan struct{})
	for range 4 {
		go func() {
			defer func() { done <- struct{}{} }()
			h.Peek()
			h.Len()
			for range h.All() {
				break
			}
		}()
	}
	for range 4 {
		<-done
	}
	// heap should be unmodified
	if h.Len() != 100 {
		t.Fatalf("Len after concurrent reads = %d, want 100", h.Len())
	}
}
