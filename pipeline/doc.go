// Package pipeline provides a generic, declarative step-execution graph for
// service-layer handlers. A Pipeline[S] chains sequential (Then), concurrent
// all-must-succeed (Parallel), and concurrent first-success (Race) stages over
// a caller-defined state struct S.
//
// Example:
//
//	type state struct {
//	    Req  *Request
//	    Data *Data
//	}
//
//	err := pipeline.New[state]().
//	    Then("validate", validate).
//	    Parallel("fetch", fetchA, fetchB).
//	    Then("save", save).
//	    Run(ctx, &state{Req: req})
package pipeline
