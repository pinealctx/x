package syncx

import (
	"context"
	"hash/maphash"
	"sync/atomic"
)

// dispatcherTask wraps a key-value pair for internal queue transport.
type dispatcherTask[K comparable, V any] struct {
	key   K
	value V
}

// dispatcherSlot holds a per-goroutine queue and its done signal.
type dispatcherSlot[K comparable, V any] struct {
	queue *BlockingQueue[dispatcherTask[K, V]]
	done  chan struct{}
}

// Dispatcher routes tasks to fixed goroutines by key hash.
// Same key always goes to the same slot goroutine, guaranteeing serial execution.
type Dispatcher[K comparable, V any] struct {
	slots    []dispatcherSlot[K, V]
	numSlots uint64
	seed     maphash.Seed
	handler  func(K, V) error
	onError  func(K, V, error)
	buffer   int
	closed   atomic.Bool
}

// DispatcherOption configures a Dispatcher.
type DispatcherOption[K comparable, V any] func(*Dispatcher[K, V])

// WithBuffer sets the per-slot queue capacity (must be >= 1).
// The default capacity is 1 (handoff semantics: Submit blocks until
// the slot goroutine takes the item). Use WithBuffer to increase
// the buffer size beyond the default.
// Panics if n < 1.
func WithBuffer[K comparable, V any](n int) DispatcherOption[K, V] {
	if n < 1 {
		panic("syncx: WithBuffer requires n >= 1")
	}
	return func(d *Dispatcher[K, V]) {
		d.buffer = n
	}
}

// WithOnError sets a callback invoked when the handler returns an error.
// The slot goroutine continues processing subsequent tasks after the callback returns.
func WithOnError[K comparable, V any](fn func(K, V, error)) DispatcherOption[K, V] {
	return func(d *Dispatcher[K, V]) {
		d.onError = fn
	}
}

// NewDispatcher creates a dispatcher with the given number of slots.
//   - slots: number of worker goroutines (must be >= 1)
//   - handler: function called for each (key, value) pair in the assigned slot goroutine
//
// handler and onError must not panic. A panic in either propagates out of the
// slot goroutine and crashes the program. Callers that need panic isolation
// should wrap their own handler with a recover.
//
// Panics if slots <= 0 or handler is nil.
func NewDispatcher[K comparable, V any](slots int, handler func(K, V) error, opts ...DispatcherOption[K, V]) *Dispatcher[K, V] {
	if slots <= 0 {
		panic("syncx: dispatcher slots must be >= 1")
	}
	if handler == nil {
		panic("syncx: dispatcher handler must not be nil")
	}

	d := &Dispatcher[K, V]{
		numSlots: uint64(slots),
		seed:     maphash.MakeSeed(),
		handler:  handler,
		buffer:   1, // default capacity: 1 (handoff semantics)
	}

	for _, opt := range opts {
		opt(d)
	}

	d.slots = make([]dispatcherSlot[K, V], slots)
	for i := range d.slots {
		s := &d.slots[i]
		s.queue = NewBlockingQueue[dispatcherTask[K, V]](d.buffer)
		s.done = make(chan struct{})
		go d.runSlot(s)
	}

	return d
}

// Submit submits a task to the slot goroutine assigned to key.
// Blocks until the slot has capacity or the dispatcher is closed.
// Use TrySubmit for non-blocking semantics.
// Returns ErrDispatcherClosed if the dispatcher is closed.
func (d *Dispatcher[K, V]) Submit(key K, value V) error {
	// Fast-path check; the authoritative check is done by BlockingQueue.Push.
	if d.closed.Load() {
		return ErrDispatcherClosed
	}
	s := &d.slots[d.slotIndex(key)]
	if err := s.queue.Push(context.Background(), dispatcherTask[K, V]{key: key, value: value}); err != nil {
		// Push can only fail with ErrQueueClosed (Background context never cancels).
		return ErrDispatcherClosed
	}
	return nil
}

// TrySubmit attempts to submit a task without blocking.
// Returns false if the slot's buffer is full or the dispatcher is closed.
func (d *Dispatcher[K, V]) TrySubmit(key K, value V) bool {
	// Fast-path check; the authoritative check is done by BlockingQueue.TryPush.
	if d.closed.Load() {
		return false
	}
	s := &d.slots[d.slotIndex(key)]
	return s.queue.TryPush(dispatcherTask[K, V]{key: key, value: value}) == nil
}

// Close signals all slots to stop and waits for pending tasks to complete.
// Idempotent — calling Close again is a no-op.
func (d *Dispatcher[K, V]) Close() {
	d.closed.Store(true)
	for i := range d.slots {
		d.slots[i].queue.Close()
	}
	for i := range d.slots {
		<-d.slots[i].done
	}
}

// slotIndex returns the slot index for the given key using maphash.
func (d *Dispatcher[K, V]) slotIndex(key K) uint64 {
	return maphash.Comparable(d.seed, key) % d.numSlots
}

// runSlot drains the slot's queue, calling the handler for each task.
// Exits when the queue is closed and fully drained.
func (d *Dispatcher[K, V]) runSlot(s *dispatcherSlot[K, V]) {
	defer close(s.done)
	for {
		t, err := s.queue.Pop(context.Background())
		if err != nil {
			return
		}
		if err := d.handler(t.key, t.value); err != nil {
			if d.onError != nil {
				d.onError(t.key, t.value, err)
			}
		}
	}
}
