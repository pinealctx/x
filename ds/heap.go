package ds

import (
	"cmp"
	"iter"
)

// Heap is a binary heap ordered by the compare function.
// compare(a, b) < 0 means a has higher priority (min-heap default).
// It is not safe for concurrent use.
type Heap[T any] struct {
	data    []T
	compare func(T, T) int
}

// NewHeap creates an empty heap with the given compare function.
// Panics if compare is nil.
func NewHeap[T any](compare func(T, T) int) *Heap[T] {
	if compare == nil {
		panic("ds: Heap requires a non-nil compare function")
	}
	return &Heap[T]{compare: compare}
}

// NewHeapFrom creates a heap from an existing slice in O(n) time.
// The slice ownership is transferred to the heap.
// Panics if compare is nil.
func NewHeapFrom[T any](compare func(T, T) int, s []T) *Heap[T] {
	if compare == nil {
		panic("ds: Heap requires a non-nil compare function")
	}
	h := &Heap[T]{data: s, compare: compare}
	h.heapify()
	return h
}

// NewMinHeap creates a min-heap for ordered types using cmp.Compare.
func NewMinHeap[T cmp.Ordered]() *Heap[T] {
	return NewHeap(cmp.Compare[T])
}

// NewMaxHeap creates a max-heap for ordered types.
func NewMaxHeap[T cmp.Ordered]() *Heap[T] {
	return NewHeap(func(a, b T) int { return cmp.Compare(b, a) })
}

// Push inserts value into the heap.
func (h *Heap[T]) Push(value T) {
	h.data = append(h.data, value)
	h.siftUp(len(h.data) - 1)
}

// Pop removes and returns the top element. Returns (zero, false) if empty.
func (h *Heap[T]) Pop() (T, bool) {
	n := len(h.data)
	if n == 0 {
		var zero T
		return zero, false
	}
	top := h.data[0]
	h.data[0] = h.data[n-1]
	var zero T
	h.data[n-1] = zero // clear for GC
	h.data = h.data[:n-1]
	if len(h.data) > 0 {
		h.siftDown(0)
	}
	return top, true
}

// Peek returns the top element without removing it. Returns (zero, false) if empty.
func (h *Heap[T]) Peek() (T, bool) {
	if len(h.data) == 0 {
		var zero T
		return zero, false
	}
	return h.data[0], true
}

// Len returns the number of elements.
func (h *Heap[T]) Len() int {
	return len(h.data)
}

// Clear removes all elements.
func (h *Heap[T]) Clear() {
	clear(h.data)
	h.data = h.data[:0]
}

// Clone returns a shallow copy of the heap. The heap property is preserved.
func (h *Heap[T]) Clone() *Heap[T] {
	cp := make([]T, len(h.data))
	copy(cp, h.data)
	return &Heap[T]{data: cp, compare: h.compare}
}

// All returns an iterator over all elements in arbitrary order. Does not modify the heap.
func (h *Heap[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range h.data {
			if !yield(v) {
				return
			}
		}
	}
}

// Drain returns an iterator that pops elements in sorted order until the heap is empty.
func (h *Heap[T]) Drain() iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			v, ok := h.Pop()
			if !ok {
				return
			}
			if !yield(v) {
				return
			}
		}
	}
}

// siftUp moves the element at index i up to restore heap property.
func (h *Heap[T]) siftUp(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if h.compare(h.data[i], h.data[parent]) < 0 {
			h.data[i], h.data[parent] = h.data[parent], h.data[i]
			i = parent
		} else {
			return
		}
	}
}

// siftDown moves the element at index i down to restore heap property.
func (h *Heap[T]) siftDown(i int) {
	n := len(h.data)
	for {
		smallest := i
		left := 2*i + 1
		right := 2*i + 2
		if left < n && h.compare(h.data[left], h.data[smallest]) < 0 {
			smallest = left
		}
		if right < n && h.compare(h.data[right], h.data[smallest]) < 0 {
			smallest = right
		}
		if smallest == i {
			return
		}
		h.data[i], h.data[smallest] = h.data[smallest], h.data[i]
		i = smallest
	}
}

// heapify builds the heap in O(n) time using bottom-up siftDown.
func (h *Heap[T]) heapify() {
	n := len(h.data)
	for i := n/2 - 1; i >= 0; i-- {
		h.siftDown(i)
	}
}
