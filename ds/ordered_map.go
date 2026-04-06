package ds

import "iter"

// entry is a node in the intrusive doubly-linked list.
type entry[K comparable, V any] struct {
	key   K
	value V
	prev  *entry[K, V]
	next  *entry[K, V]
}

// OrderedMap maintains key-value pairs in insertion order.
// Lookup, insert, and delete are all O(1). Iteration is zero-allocation.
// It is not safe for concurrent use.
type OrderedMap[K comparable, V any] struct {
	m    map[K]*entry[K, V]
	root entry[K, V] // sentinel: root.next = front, root.prev = back
}

// NewOrderedMap creates an empty OrderedMap.
func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	return NewOrderedMapWithCapacity[K, V](0)
}

// NewOrderedMapWithCapacity creates an OrderedMap with pre-allocated map capacity.
// Panics if capacity is negative.
func NewOrderedMapWithCapacity[K comparable, V any](capacity int) *OrderedMap[K, V] {
	if capacity < 0 {
		panic("ds: NewOrderedMapWithCapacity: negative capacity")
	}
	m := &OrderedMap[K, V]{
		m: make(map[K]*entry[K, V], capacity),
	}
	m.root.next = &m.root
	m.root.prev = &m.root
	return m
}

// Get returns the value associated with key, or the zero value with false if not found.
func (m *OrderedMap[K, V]) Get(key K) (V, bool) {
	if e, ok := m.m[key]; ok {
		return e.value, true
	}
	var zero V
	return zero, false
}

// Set stores the key-value pair. It returns true if the key was newly inserted,
// false if an existing key's value was updated (position unchanged).
func (m *OrderedMap[K, V]) Set(key K, value V) bool {
	if e, ok := m.m[key]; ok {
		e.value = value
		return false
	}
	e := &entry[K, V]{key: key, value: value}
	// insert at back: link between root.prev and &root
	e.prev = m.root.prev
	e.next = &m.root
	m.root.prev.next = e
	m.root.prev = e
	m.m[key] = e
	return true
}

// Has reports whether the key exists.
func (m *OrderedMap[K, V]) Has(key K) bool {
	_, ok := m.m[key]
	return ok
}

// Delete removes the key. It returns true if the key was present.
func (m *OrderedMap[K, V]) Delete(key K) bool {
	e, ok := m.m[key]
	if !ok {
		return false
	}
	// unlink: standard 4-pointer operation, no nil checks needed
	e.prev.next = e.next
	e.next.prev = e.prev
	e.prev = nil // help GC
	e.next = nil
	delete(m.m, key)
	return true
}

// Len returns the number of key-value pairs.
func (m *OrderedMap[K, V]) Len() int {
	return len(m.m)
}

// Clear removes all key-value pairs, keeping the underlying map storage.
func (m *OrderedMap[K, V]) Clear() {
	clear(m.m)
	m.root.next = &m.root
	m.root.prev = &m.root
}

// All returns an iterator over key-value pairs in insertion order.
func (m *OrderedMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for e := m.root.next; e != &m.root; e = e.next {
			if !yield(e.key, e.value) {
				return
			}
		}
	}
}

// Backward returns an iterator over key-value pairs in reverse insertion order.
func (m *OrderedMap[K, V]) Backward() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for e := m.root.prev; e != &m.root; e = e.prev {
			if !yield(e.key, e.value) {
				return
			}
		}
	}
}

// Keys returns all keys in insertion order.
func (m *OrderedMap[K, V]) Keys() []K {
	keys := make([]K, 0, len(m.m))
	for e := m.root.next; e != &m.root; e = e.next {
		keys = append(keys, e.key)
	}
	return keys
}

// Clone returns a shallow copy of the map. The insertion order is preserved.
func (m *OrderedMap[K, V]) Clone() *OrderedMap[K, V] {
	c := NewOrderedMapWithCapacity[K, V](m.Len())
	for e := m.root.next; e != &m.root; e = e.next {
		c.Set(e.key, e.value)
	}
	return c
}

// Values returns all values in insertion order.
func (m *OrderedMap[K, V]) Values() []V {
	vals := make([]V, 0, len(m.m))
	for e := m.root.next; e != &m.root; e = e.next {
		vals = append(vals, e.value)
	}
	return vals
}
