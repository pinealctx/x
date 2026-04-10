package syncx

import (
	"context"
	"sync"

	"github.com/pinealctx/x/panicx"
)

// Race runs fns concurrently and returns the first successful result.
// If all fns fail, Race returns the last error written by any goroutine
// (goroutine scheduling order, not submission order).
// The provided ctx is passed to each fn; once one fn succeeds,
// the derived context passed to remaining fns is canceled.
// Race waits for all goroutines to finish before returning to avoid leaks.
// If fns is empty, Race returns the zero value and nil.
func Race[T any](ctx context.Context, fns ...func(ctx context.Context) (T, error)) (T, error) {
	if len(fns) == 0 {
		var zero T
		return zero, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		once      sync.Once
		wg        sync.WaitGroup
		succeeded bool
		winVal    T
		mu        sync.Mutex // guards lastErr
		lastErr   error
	)

	wg.Add(len(fns))
	for _, fn := range fns {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					lastErr = panicx.NewPanicError(r)
					mu.Unlock()
				}
			}()
			val, err := fn(ctx)
			if err == nil {
				once.Do(func() {
					succeeded = true
					winVal = val
					cancel()
				})
				return
			}
			mu.Lock()
			lastErr = err
			mu.Unlock()
		}()
	}

	wg.Wait()

	if succeeded {
		return winVal, nil
	}
	var zero T
	return zero, lastErr
}
