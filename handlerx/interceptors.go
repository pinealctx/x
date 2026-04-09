package handlerx

import (
	"context"
	"fmt"
	"time"

	"github.com/pinealctx/x/errorx"
)

// frmTag is a phantom type for domain-isolated sentinel errors in the frm package.
type frmTag struct{}

// Error is the sentinel error type for the frm package.
type Error = errorx.Sentinel[frmTag]

// ErrPanicRecovered is returned by WithRecovery when a downstream handler panics.
// Callers can detect a recovered panic with errors.Is(err, frm.ErrPanicRecovered).
//
// Note: WithRecovery does not suppress runtime.Goexit (e.g. called by
// testing.T.FailNow); only panics are caught.
var ErrPanicRecovered = Error("frm: panic recovered")

// WithTimeout returns an Interceptor that applies a timeout to the request
// context. The timeout is enforced per call.
func WithTimeout[Req, Resp any](d time.Duration) Interceptor[Req, Resp] {
	return func(ctx context.Context, req Req, next Handler[Req, Resp]) (Resp, error) {
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return next(ctx, req)
	}
}

// WithRecovery returns an Interceptor that catches panics from downstream
// handlers and converts them to errors, preventing goroutine crashes.
// The returned error wraps ErrPanicRecovered and can be detected with
// errors.Is(err, frm.ErrPanicRecovered).
//
// Note: runtime.Goexit is not a panic and is not caught by WithRecovery.
func WithRecovery[Req, Resp any]() Interceptor[Req, Resp] {
	return func(ctx context.Context, req Req, next Handler[Req, Resp]) (resp Resp, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%w: %v", ErrPanicRecovered, r)
			}
		}()
		return next(ctx, req)
	}
}
