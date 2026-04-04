package ds

import "iter"

// Set is an unordered collection of unique elements.
// It is not safe for concurrent use.
type Set[T comparable] struct {
	set map[T]struct{}
}

// NewSet creates a Set initialized with the given values. Duplicates are ignored.
func NewSet[T comparable](vals ...T) *Set[T] {
	s := NewSetWithCapacity[T](len(vals))
	for _, v := range vals {
		s.set[v] = struct{}{}
	}
	return s
}

// NewSetWithCapacity creates an empty Set with pre-allocated capacity.
// Panics if n is negative.
func NewSetWithCapacity[T comparable](n int) *Set[T] {
	if n < 0 {
		panic("ds: NewSetWithCapacity: negative capacity")
	}
	return &Set[T]{set: make(map[T]struct{}, n)}
}

// Add inserts val. It returns true if the element was newly added.
func (s *Set[T]) Add(val T) bool {
	if _, exists := s.set[val]; exists {
		return false
	}
	s.set[val] = struct{}{}
	return true
}

// Remove removes val. It returns true if the element was present.
func (s *Set[T]) Remove(val T) bool {
	if _, exists := s.set[val]; !exists {
		return false
	}
	delete(s.set, val)
	return true
}

// Contains reports whether val is in the set.
func (s *Set[T]) Contains(val T) bool {
	_, ok := s.set[val]
	return ok
}

// Len returns the number of elements.
func (s *Set[T]) Len() int {
	return len(s.set)
}

// Clear removes all elements.
func (s *Set[T]) Clear() {
	clear(s.set)
}

// Union returns a new Set containing elements from both s and other.
func (s *Set[T]) Union(other *Set[T]) *Set[T] {
	result := NewSetWithCapacity[T](s.Len() + other.Len())
	for v := range s.set {
		result.set[v] = struct{}{}
	}
	for v := range other.set {
		result.set[v] = struct{}{}
	}
	return result
}

// Intersect returns a new Set containing elements present in both s and other.
func (s *Set[T]) Intersect(other *Set[T]) *Set[T] {
	// iterate over the smaller set
	small, large := s, other
	if small.Len() > large.Len() {
		small, large = large, small
	}
	result := NewSetWithCapacity[T](small.Len())
	for v := range small.set {
		if _, ok := large.set[v]; ok {
			result.set[v] = struct{}{}
		}
	}
	return result
}

// Difference returns a new Set containing elements in s but not in other.
func (s *Set[T]) Difference(other *Set[T]) *Set[T] {
	result := NewSetWithCapacity[T](s.Len())
	for v := range s.set {
		if _, ok := other.set[v]; !ok {
			result.set[v] = struct{}{}
		}
	}
	return result
}

// SymmetricDifference returns a new Set containing elements in s or other but not both.
func (s *Set[T]) SymmetricDifference(other *Set[T]) *Set[T] {
	result := NewSetWithCapacity[T](s.Len() + other.Len())
	for v := range s.set {
		if _, ok := other.set[v]; !ok {
			result.set[v] = struct{}{}
		}
	}
	for v := range other.set {
		if _, ok := s.set[v]; !ok {
			result.set[v] = struct{}{}
		}
	}
	return result
}

// Equal reports whether s and other contain the same elements.
func (s *Set[T]) Equal(other *Set[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	for v := range s.set {
		if _, ok := other.set[v]; !ok {
			return false
		}
	}
	return true
}

// IsSubset reports whether all elements of s are in other.
func (s *Set[T]) IsSubset(other *Set[T]) bool {
	if s.Len() > other.Len() {
		return false
	}
	for v := range s.set {
		if _, ok := other.set[v]; !ok {
			return false
		}
	}
	return true
}

// IsSuperset reports whether all elements of other are in s.
func (s *Set[T]) IsSuperset(other *Set[T]) bool {
	return other.IsSubset(s)
}

// All returns an iterator over the elements. Order is not guaranteed.
func (s *Set[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range s.set {
			if !yield(v) {
				return
			}
		}
	}
}

// ToSlice returns the elements as a slice. Order is not guaranteed.
func (s *Set[T]) ToSlice() []T {
	result := make([]T, 0, len(s.set))
	for v := range s.set {
		result = append(result, v)
	}
	return result
}

// Clone returns a shallow copy of the set.
func (s *Set[T]) Clone() *Set[T] {
	result := NewSetWithCapacity[T](s.Len())
	for v := range s.set {
		result.set[v] = struct{}{}
	}
	return result
}
