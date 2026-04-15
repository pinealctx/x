// Package ds provides generic data structure implementations. All containers
// are non-concurrent-safe; use external synchronization when sharing across
// goroutines.
//
// OrderedMap[K,V] preserves insertion order with O(1) access and zero-allocation
// iteration. Set[T] offers set algebra (union, intersection, difference) and
// relation checks (subset, disjoint). BiMap[K,V] provides bidirectional O(1)
// lookup. Stack[T] and Heap[T] offer LIFO and priority-queue semantics
// respectively. SortedMap[K,V] combines O(1) random access by key with O(log n)
// ordered iteration; the sort key and the map lookup key can be different fields
// of V.
package ds
