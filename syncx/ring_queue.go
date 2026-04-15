package syncx

import (
	"context"
	"sync"
)

// RingQueue is a generic concurrent ring queue that discards the oldest item when full.
// Push and PushEvict never block — if the queue is full, the oldest item is evicted.
// Pop blocks when empty, supporting context cancellation.
// After Close(), Pop continues returning remaining items then returns ErrQueueClosed.
// After CloseNow(), all operations immediately return ErrQueueClosed or are silently discarded.
type RingQueue[T any] struct {
	mu    sync.Mutex
	cond  *sync.Cond
	ring  ringBuf[T]
	state closedState
}

// NewRingQueue creates a RingQueue with the given capacity.
// Panics if capacity < 1.
func NewRingQueue[T any](capacity int) *RingQueue[T] {
	if capacity < 1 {
		panic("syncx: NewRingQueue: capacity must be >= 1")
	}
	q := &RingQueue[T]{
		ring: newRingBuf[T](capacity),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Push adds an item to the queue. If the queue is full, the oldest item is silently discarded.
// After Close/CloseNow, the item is silently discarded.
func (q *RingQueue[T]) Push(item T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.state != openState {
		return
	}

	if q.ring.full() {
		q.ring.pop()
	}

	q.ring.push(item)
	q.cond.Broadcast()
}

// PushEvict adds an item and returns the evicted item if the queue was full.
// Returns (zero, false) if no item was evicted.
// After Close/CloseNow, the item is silently discarded and (zero, false) is returned.
func (q *RingQueue[T]) PushEvict(item T) (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.state != openState {
		var zero T
		return zero, false
	}

	if q.ring.full() {
		old := q.ring.pop()
		q.ring.push(item)
		q.cond.Broadcast()
		return old, true
	}

	q.ring.push(item)
	q.cond.Broadcast()
	var zero T
	return zero, false
}

// Pop removes and returns the front item, blocking if empty.
// After Close(), returns remaining items, then ErrQueueClosed.
// After CloseNow(), immediately returns ErrQueueClosed.
func (q *RingQueue[T]) Pop(ctx context.Context) (T, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.ring.empty() {
		if q.state != openState {
			var zero T
			return zero, ErrQueueClosed
		}
		if err := ctx.Err(); err != nil {
			var zero T
			return zero, err
		}
		if err := waitCond(ctx, q.cond); err != nil {
			var zero T
			return zero, err
		}
	}

	if q.state == closedNow {
		var zero T
		return zero, ErrQueueClosed
	}

	item := q.ring.pop()
	q.cond.Broadcast()
	return item, nil
}

// TryPop removes and returns the front item without blocking.
// Returns ErrQueueEmpty if empty, ErrQueueClosed if closed.
func (q *RingQueue[T]) TryPop() (T, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.state == closedNow {
		var zero T
		return zero, ErrQueueClosed
	}
	if q.ring.empty() {
		var zero T
		if q.state == closedDrain {
			return zero, ErrQueueClosed
		}
		return zero, ErrQueueEmpty
	}

	item := q.ring.pop()
	q.cond.Broadcast()
	return item, nil
}

// Close closes the queue in drain mode.
// Push and PushEvict silently discard items.
// Pop continues returning remaining items, then returns ErrQueueClosed.
// Idempotent — calling Close or CloseNow again is a no-op.
func (q *RingQueue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.state != openState {
		return
	}
	q.state = closedDrain
	q.cond.Broadcast()
}

// CloseNow closes the queue immediately, discarding all remaining items.
// Pop immediately returns ErrQueueClosed.
// Idempotent — calling Close or CloseNow again is a no-op.
func (q *RingQueue[T]) CloseNow() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.state != openState {
		return
	}
	q.state = closedNow
	q.cond.Broadcast()
}

// Len returns the number of items currently in the queue.
func (q *RingQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.ring.len()
}

// Peek returns the front item without removing it.
// Returns false if the queue is empty.
func (q *RingQueue[T]) Peek() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.ring.peek()
}
