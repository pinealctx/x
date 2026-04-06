// Package retryx provides a generic retry mechanism with composable backoff
// strategies. It has zero external dependencies.
//
// Do[T] retries a function with configurable attempts, backoff, retry
// condition, and per-attempt callbacks. BackoffStrategy is composable:
// Exponential and Fixed bases can be decorated with WithJitter and WithMaxWait
// to control variance and ceiling.
package retryx
