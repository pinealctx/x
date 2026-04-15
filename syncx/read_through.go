package syncx

import "context"

// Cache is the minimal interface that ReadThrough delegates storage to.
// Implementations must be safe for concurrent use from multiple goroutines.
// TTL, eviction, and capacity management are the responsibility of the
// implementation; ReadThrough only calls Get and Set.
type Cache[K comparable, V any] interface {
	// Get retrieves a value by key. The second return value reports whether
	// the key was found.
	Get(key K) (V, bool)
	// Set stores a value under the given key.
	Set(key K, value V)
}

// ReadThrough wraps a [Cache] with per-key stampede protection using
// [KeyedMutex]. On a cache miss it calls the caller-provided loader, then
// populates the cache before returning. Loader errors are not cached.
//
// Callers should prefer immutable value types for V. When V is a mutable
// reference type (pointer, slice, or map), the caller is responsible for
// ensuring that cached values are not modified after being returned.
//
// The zero value is not usable; construct with [NewReadThrough].
type ReadThrough[K comparable, V any] struct {
	cache  Cache[K, V]
	loader func(ctx context.Context, key K) (V, error)
	km     *KeyedMutex[K]
}

// NewReadThrough creates a ReadThrough that delegates cache storage to c and
// loads missing values through loader.
// Panics if c or loader is nil.
func NewReadThrough[K comparable, V any](
	c Cache[K, V],
	loader func(ctx context.Context, key K) (V, error),
) *ReadThrough[K, V] {
	if c == nil {
		panic("syncx: NewReadThrough: cache must not be nil")
	}
	if loader == nil {
		panic("syncx: NewReadThrough: loader must not be nil")
	}
	return &ReadThrough[K, V]{
		cache:  c,
		loader: loader,
		km:     NewKeyedMutex[K](),
	}
}

// Get returns the value for key, loading and caching it on miss.
// On cache hit the loader is not invoked. On miss the loader is called
// under per-key exclusive lock with double-check, so concurrent callers for
// the same key block until the first load completes. Loader errors are
// propagated to the caller and are not cached.
//
// The per-key lock acquisition is not context-aware: if ctx is already
// canceled when the lock is contended, the goroutine still blocks until
// the current holder releases it. Context cancellation only affects the
// loader call itself.
func (rt *ReadThrough[K, V]) Get(ctx context.Context, key K) (V, error) {
	// Fast path: cache hit without any lock.
	if v, ok := rt.cache.Get(key); ok {
		return v, nil
	}

	// Slow path: per-key lock + double-check.
	unlock := rt.km.Lock(key)
	defer unlock()

	// Double-check after acquiring lock.
	if v, ok := rt.cache.Get(key); ok {
		return v, nil
	}

	// Load from source.
	v, err := rt.loader(ctx, key)
	if err != nil {
		var zero V
		return zero, err
	}

	// Populate cache.
	rt.cache.Set(key, v)
	return v, nil
}
