// Package syncx provides generic synchronization primitives that extend
// the standard [sync] package with type-safe, zero-cast APIs built on Go
// generics.
//
// Per-key locking: KeyedMutex[K] provides per-key mutual exclusion with
// automatic entry cleanup via reference counting. KeyedLocker[K] extends
// this with read/write semantics. Both use RAII-style unlock functions.
//
// Concurrent queues: BlockingQueue[T] and RingQueue[T] offer context-aware
// blocking, close semantics, and non-blocking try variants.
//
// Cache-aside: ReadThrough[K,V] wraps a [Cache] backend with per-key
// stampede protection via double-checked locking.
//
// Object pooling: Pool[T] is a type-safe wrapper around [sync.Pool] with
// optional reset.
//
// Concurrency patterns: Dispatcher[K,V] routes keyed work to fixed
// goroutine slots by hash. SingleFlight[K,V] deduplicates concurrent calls
// for the same key. Group[T] collects results from multiple goroutines in
// submission order with panic recovery.
package syncx
