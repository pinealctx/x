package panicx

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
)

// ErrPanic is the sentinel for all recovered panics.
// Use errors.Is to detect, errors.As to access Value and Stack.
var ErrPanic = errors.New("panic recovered")

// PanicError is created when a recovered panic is converted to an error.
// It carries the original panic value and the call stack at the point of recovery.
// Must always be used as a pointer — contains sync.Once, must not be copied.
type PanicError struct {
	// Value is the value passed to panic().
	Value any
	pcs   []uintptr
	once  sync.Once
	stack []string
}

// NewPanicError creates a PanicError from a recovered panic value.
// Must be called directly inside the defer func() { if r := recover() } block.
// The first frame in Stack() is the defer func itself (recover site).
func NewPanicError(r any) *PanicError {
	return captureStack(r, 1) // +1 to skip NewPanicError itself
}

// NewPanicErrorSkip is like NewPanicError but skips additional frames.
// skip=0 identifies the caller of NewPanicErrorSkip.
// Add 1 per extra wrapping layer between the defer and this call.
func NewPanicErrorSkip(r any, skip int) *PanicError {
	return captureStack(r, skip+1) // +1 to skip NewPanicErrorSkip itself
}

// captureStack captures the call stack and returns a PanicError.
// skip=0 identifies the caller of captureStack.
func captureStack(r any, skip int) *PanicError {
	const maxDepth = 64
	var buf [maxDepth]uintptr
	// +2: runtime.Callers + captureStack itself.
	n := runtime.Callers(skip+2, buf[:])
	pcs := make([]uintptr, n)
	copy(pcs, buf[:n])
	return &PanicError{Value: r, pcs: pcs}
}

// Error implements the error interface.
func (e *PanicError) Error() string {
	return fmt.Sprintf("panic recovered: %v", e.Value)
}

// Is reports whether this error matches target.
// Supports errors.Is(err, panicx.ErrPanic).
func (e *PanicError) Is(target error) bool {
	return target == ErrPanic
}

// Stack returns the formatted call stack at the point of the panic.
// Each element is one frame: "func.Name\n\tfile.go:line".
// Computed lazily on first call and cached. Safe for concurrent use.
// The last frame (runtime.main/runtime.goexit) is omitted.
func (e *PanicError) Stack() []string {
	e.once.Do(func() {
		frames := runtime.CallersFrames(e.pcs)
		var result []string
		for {
			f, more := frames.Next()
			result = append(result, fmt.Sprintf("%s\n\t%s:%d", f.Function, f.File, f.Line))
			if !more {
				break
			}
		}
		// Drop the last frame (runtime.main or runtime.goexit), consistent with zap.
		if len(result) > 0 {
			result = result[:len(result)-1]
		}
		e.stack = result
	})
	return e.stack
}
