package syncx

import (
	"context"
	"sync"
)

// closedState tracks the closure state of a queue.
type closedState int

const (
	// openState is intentionally the zero value so that queue structs default to
	// the open state without explicit initialization.
	openState   closedState = iota
	closedDrain             // Close() called — drain remaining items
	closedNow               // CloseNow() called — discard remaining items
)

// ringBuf is a generic ring buffer for internal use by queue implementations.
// It provides pure buffer operations without concurrency concerns — the caller
// is responsible for synchronization.
type ringBuf[T any] struct {
	buf   []T
	head  int
	tail  int
	count int
}

func newRingBuf[T any](capacity int) ringBuf[T] {
	return ringBuf[T]{buf: make([]T, capacity)}
}

func (b *ringBuf[T]) push(item T) {
	b.buf[b.tail] = item
	b.tail = (b.tail + 1) % len(b.buf)
	b.count++
}

// pop removes and returns the front item. Caller must ensure the buffer is non-empty.
func (b *ringBuf[T]) pop() T {
	item := b.buf[b.head]
	var zero T
	b.buf[b.head] = zero // clear reference to help GC
	b.head = (b.head + 1) % len(b.buf)
	b.count--
	return item
}

func (b *ringBuf[T]) peek() (T, bool) {
	if b.count == 0 {
		var zero T
		return zero, false
	}
	return b.buf[b.head], true
}

func (b *ringBuf[T]) len() int    { return b.count }
func (b *ringBuf[T]) full() bool  { return b.count == len(b.buf) }
func (b *ringBuf[T]) empty() bool { return b.count == 0 }

// waitCond blocks on c.Wait() while respecting context cancellation.
// Uses context.AfterFunc to trigger Broadcast only when the context is canceled,
// avoiding a goroutine-per-wait on the normal (non-canceled) path.
// Must be called with c.L locked.
func waitCond(ctx context.Context, c *sync.Cond) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	stop := context.AfterFunc(ctx, func() {
		c.Broadcast()
	})
	defer stop()

	c.Wait()
	return ctx.Err()
}
