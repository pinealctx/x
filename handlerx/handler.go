package handlerx

import "context"

// Handler is a generic RPC handler function.
type Handler[Req, Resp any] func(ctx context.Context, req Req) (Resp, error)

// Interceptor wraps a Handler, allowing logic to run before and after the next
// handler in the chain.
type Interceptor[Req, Resp any] func(ctx context.Context, req Req, next Handler[Req, Resp]) (Resp, error)

// Chain composes interceptors around a handler and returns a new Handler.
// Execution order: interceptors[0] is outermost, handler is innermost.
// Calling Chain with no interceptors returns the handler unchanged.
func Chain[Req, Resp any](handler Handler[Req, Resp], interceptors ...Interceptor[Req, Resp]) Handler[Req, Resp] {
	n := len(interceptors)
	if n == 0 {
		return handler
	}
	// Build the chain iteratively from the inside out to avoid per-call
	// recursion overhead.
	h := handler
	for i := n - 1; i >= 0; i-- {
		cur, next := interceptors[i], h
		h = func(ctx context.Context, req Req) (Resp, error) {
			return cur(ctx, req, next)
		}
	}
	return h
}
