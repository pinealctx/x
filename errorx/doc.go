// Package errorx provides generic error handling primitives with code-based
// classification and type-level domain isolation. It has zero external
// dependencies.
//
// Error[Code] wraps an error with a typed integer code and optional cause
// chain. Chain-aware query functions (IsCode, ContainsCode) traverse the
// [errors.Unwrap] chain, penetrating across different Code type nodes.
//
// Sentinel[D] uses a phantom type parameter to create domain-isolated sentinel
// errors that are distinguishable at compile time while remaining plain strings
// at runtime.
package errorx
