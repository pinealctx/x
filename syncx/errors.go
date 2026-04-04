package syncx

import "github.com/pinealctx/x/errorx"

// syncxTag is a phantom type used for domain-isolated sentinel errors in the syncx package.
type syncxTag struct{}

// SyncError is the base sentinel error type for the syncx package.
type SyncError = errorx.Sentinel[syncxTag]

// Queue sentinel errors.
var (
	ErrQueueClosed = SyncError("queue closed")
	ErrQueueFull   = SyncError("queue full")
	ErrQueueEmpty  = SyncError("queue empty")
)
