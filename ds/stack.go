package ds

import "iter"

// Stack is a LIFO stack backed by a dynamic array.
// It is not safe for concurrent use.
type Stack[T any] struct {
	data []T
}

// NewStack creates an empty Stack.
func NewStack[T any]() *Stack[T] {
	return &Stack[T]{}
}

// NewStackWithCapacity creates a Stack with pre-allocated capacity.
// Panics if n is negative.
func NewStackWithCapacity[T any](n int) *Stack[T] {
	if n < 0 {
		panic("ds: NewStackWithCapacity: negative capacity")
	}
	return &Stack[T]{data: make([]T, 0, n)}
}

// Push adds item to the top of the stack.
func (s *Stack[T]) Push(item T) {
	s.data = append(s.data, item)
}

// Pop removes and returns the top element. Returns (zero, false) if empty.
func (s *Stack[T]) Pop() (T, bool) {
	n := len(s.data)
	if n == 0 {
		var zero T
		return zero, false
	}
	top := s.data[n-1]
	var zero T
	s.data[n-1] = zero // clear for GC
	s.data = s.data[:n-1]
	return top, true
}

// Peek returns the top element without removing it. Returns (zero, false) if empty.
func (s *Stack[T]) Peek() (T, bool) {
	if len(s.data) == 0 {
		var zero T
		return zero, false
	}
	return s.data[len(s.data)-1], true
}

// Len returns the number of elements.
func (s *Stack[T]) Len() int {
	return len(s.data)
}

// Clear removes all elements, keeping the underlying slice storage.
func (s *Stack[T]) Clear() {
	clear(s.data)
	s.data = s.data[:0]
}

// Clone returns a shallow copy of the stack. The element order is preserved.
func (s *Stack[T]) Clone() *Stack[T] {
	cp := make([]T, len(s.data))
	copy(cp, s.data)
	return &Stack[T]{data: cp}
}

// All returns an iterator over elements in LIFO order (top to bottom).
func (s *Stack[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := len(s.data) - 1; i >= 0; i-- {
			if !yield(s.data[i]) {
				return
			}
		}
	}
}
