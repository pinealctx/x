package ds

import (
	"iter"

	"github.com/tidwall/btree"
)

// SortedMap maintains values sorted by a caller-defined order.
// Backed by a hash map for O(1) random access and a B-tree for O(log n) ordered ops.
// The sort key and the map lookup key can be different fields of V.
// It is not safe for concurrent use.
//
// V is stored by value in both the hash map and the B-tree nodes. When V is a
// large struct, each Set/Delete incurs a value copy inside the B-tree. In that
// case, prefer using SortedMap[K, *V] to avoid the copy overhead.
//
// SortedMap does not provide Clone(), Keys(), or Values() because the sort key
// is extracted from V via a caller-supplied function, making generic cloning
// ambiguous (should the B-tree be rebuilt?), and ordered slices are better
// obtained via the Ascend/Descend iterators.
type SortedMap[K comparable, V any] struct {
	m    map[K]V
	tree *btree.BTreeG[V]
	key  func(V) K
}

// NewSortedMap creates a SortedMap. key extracts the map lookup key from V;
// less defines the B-tree sort order. Both key and less must not be nil.
func NewSortedMap[K comparable, V any](key func(V) K, less func(V, V) bool) *SortedMap[K, V] {
	if key == nil {
		panic("ds: NewSortedMap: key must not be nil")
	}
	if less == nil {
		panic("ds: NewSortedMap: less must not be nil")
	}
	return &SortedMap[K, V]{
		m:    make(map[K]V),
		tree: btree.NewBTreeG(less),
		key:  key,
	}
}

// Get returns the value associated with key and whether it was found.
// If key is not present, it returns the zero value of V and false.
func (m *SortedMap[K, V]) Get(key K) (V, bool) {
	v, ok := m.m[key]
	return v, ok
}

// Has reports whether key exists.
func (m *SortedMap[K, V]) Has(key K) bool {
	_, ok := m.m[key]
	return ok
}

// Set inserts or updates val. Returns true if val was newly inserted,
// false if an existing entry with the same key was replaced.
// When updating, the B-tree always performs a Delete+Set pair regardless of
// whether the sort key changed.
func (m *SortedMap[K, V]) Set(val V) bool {
	k := m.key(val)
	old, exists := m.m[k]
	if exists {
		m.tree.Delete(old)
	}
	m.m[k] = val
	m.tree.Set(val)
	return !exists
}

// Delete removes the entry with the given key. Returns true if the entry existed.
func (m *SortedMap[K, V]) Delete(key K) bool {
	old, ok := m.m[key]
	if !ok {
		return false
	}
	delete(m.m, key)
	m.tree.Delete(old)
	return true
}

// Len returns the number of entries.
func (m *SortedMap[K, V]) Len() int {
	return len(m.m)
}

// Clear removes all entries.
func (m *SortedMap[K, V]) Clear() {
	clear(m.m)
	m.tree.Clear()
}

// Ascend returns an iterator over all values in ascending order.
func (m *SortedMap[K, V]) Ascend() iter.Seq[V] {
	return func(yield func(V) bool) {
		it := m.tree.Iter()
		defer it.Release()
		for ok := it.First(); ok; ok = it.Next() {
			if !yield(it.Item()) {
				return
			}
		}
	}
}

// AscendFrom returns an iterator over values >= pivot in ascending order.
func (m *SortedMap[K, V]) AscendFrom(pivot V) iter.Seq[V] {
	return func(yield func(V) bool) {
		it := m.tree.Iter()
		defer it.Release()
		if !it.Seek(pivot) {
			return
		}
		for {
			if !yield(it.Item()) {
				return
			}
			if !it.Next() {
				return
			}
		}
	}
}

// AscendAfter returns an iterator over values > pivot in ascending order.
func (m *SortedMap[K, V]) AscendAfter(pivot V) iter.Seq[V] {
	return func(yield func(V) bool) {
		it := m.tree.Iter()
		defer it.Release()
		// Seek positions at first >= pivot; skip elements equal to pivot.
		if !it.Seek(pivot) {
			return
		}
		// Skip all items where !(pivot < item), i.e. item <= pivot.
		for !m.tree.Less(pivot, it.Item()) {
			if !it.Next() {
				return
			}
		}
		for {
			if !yield(it.Item()) {
				return
			}
			if !it.Next() {
				return
			}
		}
	}
}

// Descend returns an iterator over all values in descending order.
func (m *SortedMap[K, V]) Descend() iter.Seq[V] {
	return func(yield func(V) bool) {
		it := m.tree.Iter()
		defer it.Release()
		for ok := it.Last(); ok; ok = it.Prev() {
			if !yield(it.Item()) {
				return
			}
		}
	}
}

// DescendFrom returns an iterator over values <= pivot in descending order.
func (m *SortedMap[K, V]) DescendFrom(pivot V) iter.Seq[V] {
	return func(yield func(V) bool) {
		it := m.tree.Iter()
		defer it.Release()
		if it.Seek(pivot) {
			// Seek landed on first >= pivot. If item > pivot, step back.
			if m.tree.Less(pivot, it.Item()) {
				if !it.Prev() {
					return
				}
			}
		} else {
			// No item >= pivot; all items are <= pivot, start from last.
			if !it.Last() {
				return
			}
		}
		for {
			if !yield(it.Item()) {
				return
			}
			if !it.Prev() {
				return
			}
		}
	}
}

// DescendBefore returns an iterator over values < pivot in descending order.
func (m *SortedMap[K, V]) DescendBefore(pivot V) iter.Seq[V] {
	return func(yield func(V) bool) {
		it := m.tree.Iter()
		defer it.Release()
		if it.Seek(pivot) {
			// Step back past all items >= pivot.
			if !it.Prev() {
				return
			}
		} else {
			// No item >= pivot; all items are < pivot, start from last.
			if !it.Last() {
				return
			}
		}
		for {
			if !yield(it.Item()) {
				return
			}
			if !it.Prev() {
				return
			}
		}
	}
}
