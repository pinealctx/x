// Package ctxv provides type-safe context values through generic keys,
// eliminating the boilerplate of type assertions with [context.Value].
// It has zero external dependencies.
//
// Key[T] is a zero-value generic key type. NewKey creates a unique key bound
// to type T, and WithValue/Value/MustValue provide the standard context
// read-write operations with compile-time type safety.
package ctxv
