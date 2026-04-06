package ctxv

import (
	"context"
	"fmt"
)

// Key is a typed context key that stores values of type T.
// Each call to [NewKey] produces a unique key, even for the same type parameter.
// The zero value is not usable; construct with [NewKey].
type Key[T any] struct {
	name string
}

// NewKey creates a new typed context key with the given name for debugging.
func NewKey[T any](name string) *Key[T] {
	return &Key[T]{name: name}
}

// WithValue stores a typed value in the context and returns a derived context.
func (k *Key[T]) WithValue(ctx context.Context, val T) context.Context {
	return context.WithValue(ctx, k, val)
}

// Value retrieves a typed value from the context.
// Returns the value and true if found, the zero value and false otherwise.
func (k *Key[T]) Value(ctx context.Context) (T, bool) {
	val, ok := ctx.Value(k).(T)
	return val, ok
}

// MustValue retrieves a typed value from the context.
// Panics if the value is not found.
// This follows the Go Must convention (e.g. template.Must, regexp.MustCompile)
// where panic signals a programming error by the caller.
func (k *Key[T]) MustValue(ctx context.Context) T {
	val, ok := k.Value(ctx)
	if !ok {
		panic(fmt.Sprintf("ctxv: key %q not found in context", k.name))
	}
	return val
}

// String returns the key name for debugging.
func (k *Key[T]) String() string {
	return k.name
}
