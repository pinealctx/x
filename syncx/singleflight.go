package syncx

import (
	"sync"

	"github.com/pinealctx/x/panicx"
)

// sfCall represents an in-flight or completed SingleFlight call.
type sfCall[V any] struct {
	wg  sync.WaitGroup
	val V
	err error
}

// SingleFlight deduplicates concurrent calls for the same key.
// Multiple goroutines requesting the same key simultaneously share a single execution.
type SingleFlight[K comparable, V any] struct {
	mu    sync.Mutex
	calls map[K]*sfCall[V]
}

// NewSingleFlight creates a new SingleFlight instance.
func NewSingleFlight[K comparable, V any]() *SingleFlight[K, V] {
	return &SingleFlight[K, V]{
		calls: make(map[K]*sfCall[V]),
	}
}

// Do executes fn for the given key if no in-flight call exists for that key.
// If a call is already in progress, it waits and shares the result.
//   - shared=false: this call executed fn
//   - shared=true:  this call shared another goroutine's result
func (sf *SingleFlight[K, V]) Do(key K, fn func() (V, error)) (v V, shared bool, err error) {
	sf.mu.Lock()
	if c, ok := sf.calls[key]; ok {
		sf.mu.Unlock()
		c.wg.Wait()
		return c.val, true, c.err
	}
	c := &sfCall[V]{}
	c.wg.Add(1)
	sf.calls[key] = c
	sf.mu.Unlock()

	// Execute fn with panic recovery to prevent blocking waiters.
	func() {
		defer func() {
			if r := recover(); r != nil {
				c.err = panicx.NewPanicError(r)
			}
			c.wg.Done()
		}()
		c.val, c.err = fn()
	}()

	// Remove from map only if this is still the active call for the key.
	// Prevents deleting a newer call created after Forget.
	sf.mu.Lock()
	if sf.calls[key] == c {
		delete(sf.calls, key)
	}
	sf.mu.Unlock()

	return c.val, false, c.err
}

// Forget removes the key from the in-flight map, allowing future Do calls
// for this key to execute fn instead of sharing an in-flight result.
func (sf *SingleFlight[K, V]) Forget(key K) {
	sf.mu.Lock()
	delete(sf.calls, key)
	sf.mu.Unlock()
}
