package ds

import "iter"

// BiMap is a bidirectional map with O(1) lookup in both directions.
// Each key maps to exactly one value and each value maps to exactly one key.
// It is not safe for concurrent use.
type BiMap[K comparable, V comparable] struct {
	forward map[K]V
	inverse map[V]K
}

// NewBiMap creates an empty BiMap.
func NewBiMap[K, V comparable]() *BiMap[K, V] {
	return &BiMap[K, V]{
		forward: make(map[K]V),
		inverse: make(map[V]K),
	}
}

// Set associates key with value. If key or value already has a mapping,
// the old mapping is removed to maintain the 1:1 invariant.
func (m *BiMap[K, V]) Set(key K, value V) {
	// remove old key→value mapping if key exists
	if oldVal, ok := m.forward[key]; ok {
		delete(m.inverse, oldVal)
	}
	// remove old value→key mapping if value exists
	if oldKey, ok := m.inverse[value]; ok {
		delete(m.forward, oldKey)
	}
	m.forward[key] = value
	m.inverse[value] = key
}

// GetByKey returns the value associated with key.
func (m *BiMap[K, V]) GetByKey(key K) (V, bool) {
	v, ok := m.forward[key]
	return v, ok
}

// GetByValue returns the key associated with value.
func (m *BiMap[K, V]) GetByValue(value V) (K, bool) {
	k, ok := m.inverse[value]
	return k, ok
}

// DeleteByKey removes the mapping for key. Returns true if the key existed.
func (m *BiMap[K, V]) DeleteByKey(key K) bool {
	val, ok := m.forward[key]
	if !ok {
		return false
	}
	delete(m.forward, key)
	delete(m.inverse, val)
	return true
}

// DeleteByValue removes the mapping for value. Returns true if the value existed.
func (m *BiMap[K, V]) DeleteByValue(value V) bool {
	key, ok := m.inverse[value]
	if !ok {
		return false
	}
	delete(m.forward, key)
	delete(m.inverse, value)
	return true
}

// Len returns the number of mappings.
func (m *BiMap[K, V]) Len() int {
	return len(m.forward)
}

// Clear removes all mappings, keeping the underlying map storage.
func (m *BiMap[K, V]) Clear() {
	clear(m.forward)
	clear(m.inverse)
}

// All returns an iterator over all key-value pairs.
func (m *BiMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range m.forward {
			if !yield(k, v) {
				return
			}
		}
	}
}

// Keys returns all keys.
func (m *BiMap[K, V]) Keys() []K {
	keys := make([]K, 0, len(m.forward))
	for k := range m.forward {
		keys = append(keys, k)
	}
	return keys
}

// Values returns all values.
func (m *BiMap[K, V]) Values() []V {
	vals := make([]V, 0, len(m.inverse))
	for v := range m.inverse {
		vals = append(vals, v)
	}
	return vals
}
