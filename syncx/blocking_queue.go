package syncx

import (
	"context"
	"sync"
)

// BlockingQueue is a generic concurrent queue with blocking and non-blocking modes.
// It uses a ring buffer backed by sync.Cond for efficient blocking.
// After Close(), Pop continues returning remaining items then returns ErrQueueClosed.
// After CloseNow(), all operations immediately return ErrQueueClosed.
type BlockingQueue[T any] struct {
	mu    sync.Mutex
	cond  *sync.Cond
	ring  ringBuf[T]
	state closedState
}

// NewBlockingQueue creates a BlockingQueue with the given capacity.
// Panics if capacity < 1.
func NewBlockingQueue[T any](capacity int) *BlockingQueue[T] {
	if capacity < 1 {
		panic("syncx: BlockingQueue capacity must be >= 1")
	}
	q := &BlockingQueue[T]{
		ring: newRingBuf[T](capacity),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Push adds an item to the queue, blocking if full.
// Returns ErrQueueClosed if the queue is closed.
// Returns the context error if ctx is canceled.
func (q *BlockingQueue[T]) Push(ctx context.Context, item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.ring.full() && q.state == openState {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := waitCond(ctx, q.cond); err != nil {
			return err
		}
	}

	if q.state != openState {
		return ErrQueueClosed
	}

	q.ring.push(item)
	q.cond.Broadcast()
	return nil
}

// Pop removes and returns the front item, blocking if empty.
// After Close(), returns remaining items, then ErrQueueClosed.
// After CloseNow(), immediately returns ErrQueueClosed.
func (q *BlockingQueue[T]) Pop(ctx context.Context) (T, error) {
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

// TryPush adds an item without blocking.
// Returns ErrQueueFull if full, ErrQueueClosed if closed.
func (q *BlockingQueue[T]) TryPush(item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.state != openState {
		return ErrQueueClosed
	}
	if q.ring.full() {
		return ErrQueueFull
	}

	q.ring.push(item)
	q.cond.Broadcast()
	return nil
}

// TryPop removes and returns the front item without blocking.
// Returns ErrQueueEmpty if empty, ErrQueueClosed if closed (and empty in drain mode).
func (q *BlockingQueue[T]) TryPop() (T, error) {
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
// Push returns ErrQueueClosed. Pop continues returning remaining items,
// then returns ErrQueueClosed when empty.
// Idempotent — calling Close or CloseNow again is a no-op.
func (q *BlockingQueue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.state != openState {
		return
	}
	q.state = closedDrain
	q.cond.Broadcast()
}

// CloseNow closes the queue immediately, discarding all remaining items.
// Both Push and Pop immediately return ErrQueueClosed.
// Idempotent — calling Close or CloseNow again is a no-op.
func (q *BlockingQueue[T]) CloseNow() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.state != openState {
		return
	}
	q.state = closedNow
	q.cond.Broadcast()
}

// Len returns the number of items currently in the queue.
func (q *BlockingQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.ring.len()
}

// Peek returns the front item without removing it.
// Returns false if the queue is empty.
func (q *BlockingQueue[T]) Peek() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.ring.peek()
}
