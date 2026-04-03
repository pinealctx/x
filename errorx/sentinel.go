package errorx

import "fmt"

// Sentinel is a generic string-based sentinel error type.
// The phantom type parameter D provides type-level domain separation
// without affecting the runtime representation — it is still just a string.
type Sentinel[D any] string

// Compile-time assertion that Sentinel implements error.
var _ error = Sentinel[struct{}]("")

// Error implements the error interface.
func (e Sentinel[D]) Error() string { return string(e) }

// NewSentinel creates a new Sentinel error with the given message.
func NewSentinel[D any](message string) Sentinel[D] {
	return Sentinel[D](message)
}

// NewSentinelf creates a new Sentinel error with a formatted message.
func NewSentinelf[D any](format string, args ...any) Sentinel[D] {
	return Sentinel[D](fmt.Sprintf(format, args...))
}
