// Package syncx provides generic synchronization primitives that extend
// the standard [sync] package. It offers per-key locking and concurrent
// queue data structures built with Go generics for type-safe, zero-cast APIs.
//
// KeyedMutex[K] provides per-key mutual exclusion with automatic entry
// cleanup via reference counting. KeyedLocker[K] extends this with
// read/write semantics. Both use RAII-style unlock functions to prevent
// mismatched Lock/Unlock calls.
//
// BlockingQueue[T] and RingQueue[T] provide concurrent queue implementations
// with context-aware blocking, close semantics, and non-blocking try variants.
package syncx
