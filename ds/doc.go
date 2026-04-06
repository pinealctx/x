// Package ds provides generic data structure implementations with zero
// external dependencies. All containers are non-concurrent-safe; use
// external synchronization when sharing across goroutines.
//
// OrderedMap[K,V] preserves insertion order with O(1) access and zero-allocation
// iteration. Set[T] offers set algebra (union, intersection, difference) and
// relation checks (subset, disjoint). BiMap[K,V] provides bidirectional O(1)
// lookup. Stack[T] and Heap[T] offer LIFO and priority-queue semantics
// respectively.
package ds
