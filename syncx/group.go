package syncx

import (
	"sync"

	"github.com/pinealctx/x/panicx"
)

// Result holds the outcome of a single goroutine spawned by [Group].
type Result[T any] struct {
	Value T
	Err   error
}

// Group collects typed results from multiple goroutines.
// Unlike errgroup which returns only the first error, Group collects all (T, error) pairs,
// making it suitable for concurrent query aggregation where all results are needed.
type Group[T any] struct {
	mu     sync.Mutex
	slots  []*Result[T]
	wg     sync.WaitGroup
	sem    chan struct{}
	waited bool
}

// NewGroup creates a new Group with the given concurrency limit.
// If limit <= 0, there is no limit (all goroutines run concurrently).
func NewGroup[T any](limit int) *Group[T] {
	g := &Group[T]{}
	if limit > 0 {
		g.sem = make(chan struct{}, limit)
	}
	return g
}

// Go spawns a goroutine to execute fn and collects its result.
// If a concurrency limit was set, Go blocks until a slot is available.
// Results are collected internally and returned by [Group.Wait] in submission order.
//
// Panics if called after [Group.Wait].
func (g *Group[T]) Go(fn func() (T, error)) {
	g.mu.Lock()
	if g.waited {
		panic("syncx: Group.Go called after Wait")
	}
	slot := new(Result[T])
	g.slots = append(g.slots, slot)
	g.wg.Add(1)
	g.mu.Unlock()

	// Acquire semaphore after recording slot to preserve submission order.
	if g.sem != nil {
		g.sem <- struct{}{}
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slot.Err = panicx.NewPanicError(r)
			}
			if g.sem != nil {
				<-g.sem
			}
			g.wg.Done()
		}()
		slot.Value, slot.Err = fn()
	}()
}

// Wait blocks until all goroutines complete and returns all results.
// Results are ordered by submission (the order Go was called).
//
// Panics if called after a previous Wait (double Wait is not allowed).
func (g *Group[T]) Wait() []Result[T] {
	g.mu.Lock()
	if g.waited {
		panic("syncx: Group.Wait called more than once")
	}
	g.waited = true
	g.mu.Unlock()

	g.wg.Wait()

	results := make([]Result[T], len(g.slots))
	for i, slot := range g.slots {
		results[i] = *slot
	}
	return results
}
