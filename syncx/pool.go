package syncx

import "sync"

// Pool is a type-safe wrapper around [sync.Pool]. It eliminates the need for
// manual type assertions by encoding the element type in the generic parameter.
//
// The zero value is not usable; construct with [NewPool].
type Pool[T any] struct {
	pool  sync.Pool
	reset func(T)
}

// NewPool creates a new Pool.
//   - create: required, called when the pool is empty to create a new object.
//   - reset:  optional (at most one), called on Put to reset the object state
//     before returning to the pool.
//
// Panics if create is nil or more than one reset function is provided.
func NewPool[T any](create func() T, reset ...func(T)) *Pool[T] {
	if create == nil {
		panic("syncx: NewPool requires a non-nil create function")
	}
	if len(reset) > 1 {
		panic("syncx: NewPool accepts at most one reset function")
	}
	var resetFn func(T)
	if len(reset) > 0 {
		resetFn = reset[0]
	}
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return create()
			},
		},
		reset: resetFn,
	}
}

// Get retrieves an object from the pool, calling create if the pool is empty.
func (p *Pool[T]) Get() T {
	// Safe by construction: only Put(T) and create() populate the pool,
	// so the type assertion can never fail.
	return p.pool.Get().(T) //nolint:forcetypeassert
}

// Put returns an object to the pool, calling reset (if set) before storing.
func (p *Pool[T]) Put(x T) {
	if p.reset != nil {
		p.reset(x)
	}
	p.pool.Put(x)
}
