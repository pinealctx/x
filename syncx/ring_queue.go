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
	buf   []T
	head  int
	tail  int
	count int
	state closedState
}

// NewRingQueue creates a RingQueue with the given capacity.
// Panics if capacity < 1.
func NewRingQueue[T any](capacity int) *RingQueue[T] {
	if capacity < 1 {
		panic("syncx: RingQueue capacity must be >= 1")
	}
	q := &RingQueue[T]{
		buf: make([]T, capacity),
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

	if q.count == len(q.buf) {
		q.evictOldest()
	}

	q.pushItem(item)
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

	if q.count == len(q.buf) {
		old := q.evictOldest()
		q.pushItem(item)
		return old, true
	}

	q.pushItem(item)
	var zero T
	return zero, false
}

// Pop removes and returns the front item, blocking if empty.
// After Close(), returns remaining items, then ErrQueueClosed.
// After CloseNow(), immediately returns ErrQueueClosed.
func (q *RingQueue[T]) Pop(ctx context.Context) (T, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.count == 0 {
		if q.state != openState {
			var zero T
			return zero, ErrQueueClosed
		}
		if err := ctx.Err(); err != nil {
			var zero T
			return zero, err
		}
		if err := q.wait(ctx); err != nil {
			var zero T
			return zero, err
		}
	}

	if q.state == closedNow {
		var zero T
		return zero, ErrQueueClosed
	}

	item := q.popItem()
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
	if q.count == 0 {
		var zero T
		if q.state == closedDrain {
			return zero, ErrQueueClosed
		}
		return zero, ErrQueueEmpty
	}

	item := q.popItem()
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
	return q.count
}

// Peek returns the front item without removing it.
// Returns false if the queue is empty.
func (q *RingQueue[T]) Peek() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.count == 0 {
		var zero T
		return zero, false
	}
	return q.buf[q.head], true
}

func (q *RingQueue[T]) evictOldest() T {
	old := q.buf[q.head]
	var zero T
	q.buf[q.head] = zero
	q.head = (q.head + 1) % len(q.buf)
	q.count--
	return old
}

func (q *RingQueue[T]) pushItem(item T) {
	q.buf[q.tail] = item
	q.tail = (q.tail + 1) % len(q.buf)
	q.count++
	q.cond.Broadcast()
}

func (q *RingQueue[T]) popItem() T {
	item := q.buf[q.head]
	var zero T
	q.buf[q.head] = zero // clear reference to help GC
	q.head = (q.head + 1) % len(q.buf)
	q.count--
	return item
}

// wait blocks on cond.Wait() while respecting context cancellation.
// Uses context.AfterFunc to trigger Broadcast only when the context is canceled.
// Must be called with q.mu locked.
func (q *RingQueue[T]) wait(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	stop := context.AfterFunc(ctx, func() {
		q.cond.Broadcast()
	})
	defer stop()

	q.cond.Wait()
	return ctx.Err()
}
