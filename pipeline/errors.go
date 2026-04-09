package pipeline

import "github.com/pinealctx/x/errorx"

// pipelineTag is a phantom type for domain-isolated sentinel errors in the pipeline package.
type pipelineTag struct{}

// pipelineError is the sentinel error type for the pipeline package.
type pipelineError = errorx.Sentinel[pipelineTag]

// ErrParallelPanic is returned by a Parallel stage when a fn panics.
// Callers can detect a recovered panic with errors.Is(err, ErrParallelPanic).
var ErrParallelPanic = pipelineError("pipeline.parallel.panic")
