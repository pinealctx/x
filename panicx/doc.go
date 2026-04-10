// Package panicx provides PanicError, a unified error type for recovered panics.
// It captures the original panic value and the call stack at the point of recovery.
//
// Usage:
//
//	defer func() {
//	    if r := recover(); r != nil {
//	        err = panicx.NewPanicError(r)
//	    }
//	}()
package panicx
