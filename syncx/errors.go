package syncx

import "github.com/pinealctx/x/errorx"

// syncxTag is a phantom type used for domain-isolated sentinel errors in the syncx package.
type syncxTag struct{}

// Error is the sentinel error type for the syncx package.
// It can be used by callers to define additional syncx-domain errors
// that are distinguishable from errors in other packages.
type Error = errorx.Sentinel[syncxTag]

// Queue sentinel errors.
var (
	ErrQueueClosed = Error("syncx.queue.closed")
	ErrQueueFull   = Error("syncx.queue.full")
	ErrQueueEmpty  = Error("syncx.queue.empty")
)

// Dispatcher sentinel errors.
var (
	ErrDispatcherClosed = Error("syncx.dispatcher.closed")
)
