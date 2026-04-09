package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/pinealctx/x/syncx"
)

// StepFunc is a single pipeline step that operates on state S.
// It receives the current context and a pointer to the shared state.
type StepFunc[S any] func(ctx context.Context, state *S) error

// stage is an internal execution unit.
type stage[S any] struct {
	label string
	run   func(ctx context.Context, state *S) error
}

// Pipeline chains stages into a sequential execution graph.
// Each stage may be a single step (Then), a concurrent all-must-succeed
// group (Parallel), or a concurrent first-success group (Race).
//
// A Pipeline is not safe for concurrent use during construction: Then,
// Parallel, and Race must not be called concurrently. Once construction
// is complete, Run may be called concurrently from multiple goroutines.
type Pipeline[S any] struct {
	stages []stage[S]
}

// New creates an empty pipeline.
func New[S any]() *Pipeline[S] {
	return &Pipeline[S]{}
}

// Then appends a sequential step. The step runs after all previous stages
// complete successfully. Errors are wrapped with the stage label.
func (p *Pipeline[S]) Then(label string, fn StepFunc[S]) *Pipeline[S] {
	p.stages = append(p.stages, stage[S]{
		label: label,
		run: func(ctx context.Context, state *S) error {
			if err := fn(ctx, state); err != nil {
				return fmt.Errorf("pipeline stage %q: %w", label, err)
			}
			return nil
		},
	})
	return p
}

// Parallel appends a stage where all fns run concurrently (All semantics).
// All fns must succeed for the pipeline to proceed. If any fn fails, the
// derived context passed to remaining fns is canceled, and the first error
// encountered is returned (wrapped with the stage label). Subsequent errors
// from other fns are discarded.
//
// Concurrent fns share the same *S pointer. If multiple fns write to the
// same field, the caller is responsible for synchronization.
//
// Parallel uses sync.WaitGroup + context.WithCancel rather than syncx.Group
// to avoid collecting all errors — only the first error is needed here.
func (p *Pipeline[S]) Parallel(label string, fns ...StepFunc[S]) *Pipeline[S] {
	p.stages = append(p.stages, stage[S]{
		label: label,
		run:   parallelRun(label, fns),
	})
	return p
}

// Race appends a stage where fns run concurrently (Race semantics).
// The first fn to succeed causes the stage to succeed; all fns failing
// causes the stage to fail. The error is wrapped with the stage label.
//
// Concurrent fns share the same *S pointer. If multiple fns write to the
// same field, the caller is responsible for synchronization.
func (p *Pipeline[S]) Race(label string, fns ...StepFunc[S]) *Pipeline[S] {
	p.stages = append(p.stages, stage[S]{
		label: label,
		run:   raceRun(label, fns),
	})
	return p
}

// Run executes all stages in order with the given state.
// It returns the first error encountered, wrapped with the stage label, or nil.
func (p *Pipeline[S]) Run(ctx context.Context, state *S) error {
	for i := range p.stages {
		if err := p.stages[i].run(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

// parallelRun builds the run func for a Parallel stage.
func parallelRun[S any](label string, fns []StepFunc[S]) func(context.Context, *S) error {
	return func(ctx context.Context, state *S) error {
		if len(fns) == 0 {
			return nil
		}

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		var (
			wg       sync.WaitGroup
			mu       sync.Mutex
			firstErr error
		)

		wg.Add(len(fns))
		for _, fn := range fns {
			go func() {
				defer wg.Done()
				var fnErr error
				func() {
					defer func() {
						if r := recover(); r != nil {
							fnErr = fmt.Errorf("%w: %v", ErrParallelPanic, r)
						}
					}()
					fnErr = fn(ctx, state)
				}()
				if fnErr != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fnErr
						cancel() // signal remaining fns
					}
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		if firstErr != nil {
			return fmt.Errorf("pipeline stage %q: %w", label, firstErr)
		}
		return nil
	}
}

// raceRun builds the run func for a Race stage.
// Race semantics are delegated to syncx.Race, which is the canonical
// implementation in this module. Injecting a race runner would add API
// complexity without benefit — syncx.Race is an internal dependency, not
// an external boundary.
// adapted is allocated per Run call because each closure must capture the
// state pointer provided at run time.
func raceRun[S any](label string, fns []StepFunc[S]) func(context.Context, *S) error {
	return func(ctx context.Context, state *S) error {
		adapted := make([]func(context.Context) (struct{}, error), len(fns))
		for i, fn := range fns {
			adapted[i] = func(ctx context.Context) (struct{}, error) {
				return struct{}{}, fn(ctx, state)
			}
		}
		_, err := syncx.Race(ctx, adapted...)
		if err != nil {
			return fmt.Errorf("pipeline stage %q: %w", label, err)
		}
		return nil
	}
}
