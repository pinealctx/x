package syncx

import "sync"

// muEntry is a reference-counted Mutex entry used by KeyedMutex.
type muEntry struct {
	mu  sync.Mutex
	ref int
}

// KeyedMutex provides per-key mutual exclusion.
// Different keys can be locked concurrently; the same key is serialized.
// Entries are created on demand and removed when their reference count drops
// to zero, so there is no memory leak for unbounded key spaces.
type KeyedMutex[K comparable] struct {
	mu      sync.Mutex
	entries map[K]*muEntry
}

// NewKeyedMutex returns an initialized KeyedMutex.
func NewKeyedMutex[K comparable]() *KeyedMutex[K] {
	return &KeyedMutex[K]{entries: make(map[K]*muEntry)}
}

// Lock acquires the mutex for key and returns an unlock function.
// The caller must invoke the returned function exactly once to release the lock.
func (km *KeyedMutex[K]) Lock(key K) func() {
	e := km.acquire(key)
	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		km.release(key, e)
	}
}

// Len returns the number of active entries currently tracked.
func (km *KeyedMutex[K]) Len() int {
	km.mu.Lock()
	n := len(km.entries)
	km.mu.Unlock()
	return n
}

func (km *KeyedMutex[K]) acquire(key K) *muEntry {
	km.mu.Lock()
	e, ok := km.entries[key]
	if !ok {
		e = &muEntry{}
		km.entries[key] = e
	}
	e.ref++
	km.mu.Unlock()
	return e
}

func (km *KeyedMutex[K]) release(key K, e *muEntry) {
	km.mu.Lock()
	e.ref--
	if e.ref == 0 {
		delete(km.entries, key)
	}
	km.mu.Unlock()
}

// rwEntry is a reference-counted RWMutex entry used by KeyedLocker.
type rwEntry struct {
	mu  sync.RWMutex
	ref int
}

// KeyedLocker provides per-key read/write locking.
// Different keys can be locked concurrently; the same key is subject to
// standard read/write mutual exclusion.
// Entries are created on demand and removed when their reference count drops
// to zero, so there is no memory leak for unbounded key spaces.
type KeyedLocker[K comparable] struct {
	mu      sync.Mutex
	entries map[K]*rwEntry
}

// NewKeyedLocker returns an initialized KeyedLocker.
func NewKeyedLocker[K comparable]() *KeyedLocker[K] {
	return &KeyedLocker[K]{entries: make(map[K]*rwEntry)}
}

// Lock acquires an exclusive write lock for key and returns an unlock function.
// The caller must invoke the returned function exactly once to release the lock.
func (kl *KeyedLocker[K]) Lock(key K) func() {
	e := kl.acquire(key)
	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		kl.release(key, e)
	}
}

// RLock acquires a shared read lock for key and returns an unlock function.
// The caller must invoke the returned function exactly once to release the lock.
func (kl *KeyedLocker[K]) RLock(key K) func() {
	e := kl.acquire(key)
	e.mu.RLock()
	return func() {
		e.mu.RUnlock()
		kl.release(key, e)
	}
}

// Len returns the number of active entries currently tracked.
func (kl *KeyedLocker[K]) Len() int {
	kl.mu.Lock()
	n := len(kl.entries)
	kl.mu.Unlock()
	return n
}

func (kl *KeyedLocker[K]) acquire(key K) *rwEntry {
	kl.mu.Lock()
	e, ok := kl.entries[key]
	if !ok {
		e = &rwEntry{}
		kl.entries[key] = e
	}
	e.ref++
	kl.mu.Unlock()
	return e
}

// release is identical in logic to KeyedMutex.release.
// Cannot be unified due to Go generics not supporting struct field constraints.
func (kl *KeyedLocker[K]) release(key K, e *rwEntry) {
	kl.mu.Lock()
	e.ref--
	if e.ref == 0 {
		delete(kl.entries, key)
	}
	kl.mu.Unlock()
}
