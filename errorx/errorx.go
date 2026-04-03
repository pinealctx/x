package errorx

import (
	"errors"
	"fmt"
)

// ErrCodeConstraint constrains error code types to int-based types.
// Convention: reserve code value 0 as "unset/default"; start iota at 1.
type ErrCodeConstraint interface {
	~int
}

// Error is a generic coded error with optional cause chain.
// When Cause is nil, it represents a leaf error.
// When Cause is non-nil, it wraps an underlying error.
type Error[Code ErrCodeConstraint] struct {
	Code    Code
	Message string
	Cause   error // nil = leaf, non-nil = wrapped
}

// Compile-time assertion that *Error implements error.
var _ error = (*Error[int])(nil)

// Error implements the error interface.
func (e *Error[Code]) Error() string { return e.Message }

// Unwrap returns the underlying cause, enabling errors.Is/As chain traversal.
func (e *Error[Code]) Unwrap() error { return e.Cause }

// New creates a leaf Error with the given code and message.
func New[Code ErrCodeConstraint](code Code, message string) *Error[Code] {
	return &Error[Code]{Code: code, Message: message}
}

// Newf creates a leaf Error with a formatted message.
func Newf[Code ErrCodeConstraint](code Code, format string, args ...any) *Error[Code] {
	return &Error[Code]{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap wraps an existing error with a code and message.
// If cause is nil, returns nil.
func Wrap[Code ErrCodeConstraint](cause error, code Code, message string) *Error[Code] {
	if cause == nil {
		return nil
	}
	return &Error[Code]{Code: code, Message: message, Cause: cause}
}

// Wrapf wraps an existing error with a code and formatted message.
// If cause is nil, returns nil.
func Wrapf[Code ErrCodeConstraint](cause error, code Code, format string, args ...any) *Error[Code] {
	if cause == nil {
		return nil
	}
	return &Error[Code]{Code: code, Message: fmt.Sprintf(format, args...), Cause: cause}
}

// IsCode reports whether the outermost Error in err's chain matching
// the same Code type has the specified code value.
// errors.As traverses the full Unwrap chain, penetrating across different Code type nodes.
func IsCode[Code ErrCodeConstraint](err error, code Code) bool {
	var ae *Error[Code]
	if !errors.As(err, &ae) {
		return false
	}
	return ae.Code == code
}

// ContainsCode reports whether any Error with the same Code type
// anywhere in the chain has the specified code value.
// Unlike IsCode which checks only the outermost match, ContainsCode
// searches the entire chain by stepping through each matched Error's Cause.
func ContainsCode[Code ErrCodeConstraint](err error, code Code) bool {
	const maxDepth = 32
	for i := 0; i < maxDepth && err != nil; i++ {
		var ae *Error[Code]
		if !errors.As(err, &ae) {
			return false
		}
		if ae.Code == code {
			return true
		}
		err = ae.Cause
	}
	return false
}
