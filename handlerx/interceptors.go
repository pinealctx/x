package handlerx

import (
	"context"
	"time"

	"github.com/pinealctx/x/panicx"
)

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
// The returned error wraps panicx.ErrPanic and can be detected with
// errors.Is(err, panicx.ErrPanic).
//
// Note: runtime.Goexit is not a panic and is not caught by WithRecovery.
func WithRecovery[Req, Resp any]() Interceptor[Req, Resp] {
	return func(ctx context.Context, req Req, next Handler[Req, Resp]) (resp Resp, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = panicx.NewPanicError(r)
			}
		}()
		return next(ctx, req)
	}
}
