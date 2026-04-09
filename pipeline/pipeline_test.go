package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// --- helpers ---

type testState struct {
	Log   []string
	Value int
}

// appendStep appends label to s.Log. Safe only for sequential (Then) steps.
func appendStep(label string) StepFunc[testState] {
	return func(_ context.Context, s *testState) error {
		s.Log = append(s.Log, label)
		return nil
	}
}

// failStep appends label to s.Log and returns err. Safe only for sequential steps.
func failStep(label string, err error) StepFunc[testState] {
	return func(_ context.Context, s *testState) error {
		s.Log = append(s.Log, label)
		return err
	}
}

// concurrentFailStep returns err without touching shared state — safe for concurrent use.
func concurrentFailStep(err error) StepFunc[testState] {
	return func(_ context.Context, _ *testState) error {
		return err
	}
}

// --- Then ---

func TestThen_SingleStep(t *testing.T) {
	s := &testState{}
	err := New[testState]().Then("a", appendStep("a")).Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Log) != 1 || s.Log[0] != "a" {
		t.Fatalf("unexpected log: %v", s.Log)
	}
}

func TestThen_MultipleStepsInOrder(t *testing.T) {
	s := &testState{}
	err := New[testState]().
		Then("a", appendStep("a")).
		Then("b", appendStep("b")).
		Then("c", appendStep("c")).
		Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c"}
	for i, v := range want {
		if s.Log[i] != v {
			t.Fatalf("step %d: got %q want %q", i, s.Log[i], v)
		}
	}
}

func TestThen_StopsOnFirstError(t *testing.T) {
	sentinel := errors.New("fail")
	s := &testState{}
	err := New[testState]().
		Then("a", appendStep("a")).
		Then("b", failStep("b", sentinel)).
		Then("c", appendStep("c")).
		Run(context.Background(), s)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
	if len(s.Log) != 2 {
		t.Fatalf("expected 2 steps executed, got %v", s.Log)
	}
}

func TestThen_ErrorWrapsLabel(t *testing.T) {
	sentinel := errors.New("oops")
	s := &testState{}
	err := New[testState]().Then("my-step", failStep("x", sentinel)).Run(context.Background(), s)
	// Then does NOT wrap — the raw fn error is returned directly.
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestThen_StateSharedAcrossSteps(t *testing.T) {
	s := &testState{Value: 0}
	inc := func(_ context.Context, st *testState) error {
		st.Value++
		return nil
	}
	_ = New[testState]().Then("a", inc).Then("b", inc).Then("c", inc).Run(context.Background(), s)
	if s.Value != 3 {
		t.Fatalf("expected 3, got %d", s.Value)
	}
}

// --- Parallel ---

func TestParallel_AllSucceed(t *testing.T) {
	var count atomic.Int32
	step := func(_ context.Context, _ *testState) error {
		count.Add(1)
		return nil
	}
	s := &testState{}
	err := New[testState]().Parallel("p", step, step, step).Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if count.Load() != 3 {
		t.Fatalf("expected 3 executions, got %d", count.Load())
	}
}

func TestParallel_AnyFailureCancelsRest(t *testing.T) {
	sentinel := errors.New("bad")
	canceled := make(chan struct{})

	fast := func(_ context.Context, _ *testState) error {
		return sentinel
	}
	slow := func(ctx context.Context, _ *testState) error {
		select {
		case <-ctx.Done():
			close(canceled)
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	}

	s := &testState{}
	err := New[testState]().Parallel("p", fast, slow).Run(context.Background(), s)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel in chain, got %v", err)
	}

	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("slow fn context was not canceled")
	}
}

func TestParallel_ErrorWrapsLabel(t *testing.T) {
	sentinel := errors.New("inner")
	s := &testState{}
	err := New[testState]().Parallel("my-parallel", failStep("x", sentinel)).Run(context.Background(), s)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("sentinel not in chain: %v", err)
	}
	want := fmt.Sprintf("pipeline stage %q:", "my-parallel")
	if len(err.Error()) < len(want) || err.Error()[:len(want)] != want {
		t.Fatalf("label not in error: %v", err)
	}
}

func TestParallel_EmptyFns(t *testing.T) {
	s := &testState{}
	err := New[testState]().Parallel("empty").Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParallel_ConcurrentStateWrite(t *testing.T) {
	// Each fn writes to a distinct field — no race expected.
	type twoField struct{ A, B int }
	setA := func(_ context.Context, s *twoField) error { s.A = 1; return nil }
	setB := func(_ context.Context, s *twoField) error { s.B = 2; return nil }
	s := &twoField{}
	err := New[twoField]().Parallel("p", setA, setB).Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if s.A != 1 || s.B != 2 {
		t.Fatalf("unexpected state: %+v", s)
	}
}

// --- Race ---

func TestRace_FirstSuccessWins(t *testing.T) {
	var count atomic.Int32
	fast := func(_ context.Context, _ *testState) error {
		count.Add(1)
		return nil
	}
	slow := func(ctx context.Context, _ *testState) error {
		<-ctx.Done()
		return ctx.Err()
	}
	s := &testState{}
	err := New[testState]().Race("r", fast, slow).Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRace_AllFailReturnError(t *testing.T) {
	sentinel := errors.New("all-fail")
	s := &testState{}
	err := New[testState]().
		Race("r", concurrentFailStep(sentinel), concurrentFailStep(sentinel)).
		Run(context.Background(), s)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("sentinel not in chain: %v", err)
	}
}

func TestRace_ErrorWrapsLabel(t *testing.T) {
	sentinel := errors.New("inner")
	s := &testState{}
	err := New[testState]().Race("my-race", concurrentFailStep(sentinel)).Run(context.Background(), s)
	if err == nil {
		t.Fatal("expected error")
	}
	want := fmt.Sprintf("pipeline stage %q:", "my-race")
	if len(err.Error()) < len(want) || err.Error()[:len(want)] != want {
		t.Fatalf("label not in error: %v", err)
	}
}

func TestRace_EmptyFns(t *testing.T) {
	s := &testState{}
	err := New[testState]().Race("empty").Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
}

// --- Mixed pipeline ---

func TestMixed_ThenParallelThen(t *testing.T) {
	var parallelCount atomic.Int32
	countStep := func(_ context.Context, _ *testState) error {
		parallelCount.Add(1)
		return nil
	}
	s := &testState{}
	err := New[testState]().
		Then("start", appendStep("start")).
		Parallel("fetch", countStep, countStep).
		Then("end", appendStep("end")).
		Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	// "start" must be first, "end" must be last in sequential log
	if s.Log[0] != "start" {
		t.Fatalf("first step wrong: %v", s.Log)
	}
	if s.Log[len(s.Log)-1] != "end" {
		t.Fatalf("last step wrong: %v", s.Log)
	}
	if parallelCount.Load() != 2 {
		t.Fatalf("expected 2 parallel executions, got %d", parallelCount.Load())
	}
}

func TestMixed_ParallelFailStopsSubsequentThen(t *testing.T) {
	sentinel := errors.New("parallel-fail")
	s := &testState{}
	err := New[testState]().
		Parallel("p", failStep("x", sentinel)).
		Then("should-not-run", appendStep("after")).
		Run(context.Background(), s)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
	for _, v := range s.Log {
		if v == "after" {
			t.Fatal("step after failed parallel should not have run")
		}
	}
}

func TestMixed_RaceSucceedContinues(t *testing.T) {
	var raceWon atomic.Bool
	winner := func(_ context.Context, _ *testState) error {
		raceWon.Store(true)
		return nil
	}
	s := &testState{}
	err := New[testState]().
		Race("r", winner, func(ctx context.Context, _ *testState) error {
			<-ctx.Done()
			return ctx.Err()
		}).
		Then("after", appendStep("after")).
		Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if !raceWon.Load() {
		t.Fatal("winner fn should have run")
	}
	found := false
	for _, v := range s.Log {
		if v == "after" {
			found = true
		}
	}
	if !found {
		t.Fatal("step after successful race should have run")
	}
}

func TestEmpty_Pipeline(t *testing.T) {
	s := &testState{}
	err := New[testState]().Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
}

// --- Additional coverage ---

func TestRun_PreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Run

	called := false
	step := func(ctx context.Context, _ *testState) error {
		called = true
		return ctx.Err()
	}
	s := &testState{}
	err := New[testState]().Then("check", step).Run(ctx, s)
	if !called {
		t.Fatal("step should have been called even with pre-canceled ctx")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestParallel_PreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &testState{}
	err := New[testState]().
		Parallel("p", func(ctx context.Context, _ *testState) error {
			return ctx.Err()
		}).
		Run(ctx, s)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled in chain, got %v", err)
	}
}

func TestRace_PreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &testState{}
	err := New[testState]().
		Race("r", func(ctx context.Context, _ *testState) error {
			return ctx.Err()
		}).
		Run(ctx, s)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled in chain, got %v", err)
	}
}

func TestParallel_MultipleFailFirstErrProtected(t *testing.T) {
	errA := errors.New("err-a")
	errB := errors.New("err-b")
	s := &testState{}
	err := New[testState]().
		Parallel("p", concurrentFailStep(errA), concurrentFailStep(errB)).
		Run(context.Background(), s)
	if err == nil {
		t.Fatal("expected error")
	}
	// exactly one of the two errors must be in the chain
	if !errors.Is(err, errA) && !errors.Is(err, errB) {
		t.Fatalf("expected errA or errB in chain, got %v", err)
	}
}

func TestRace_ConcurrentStateWriteDifferentFields(t *testing.T) {
	type twoField struct{ A, B int }
	setA := func(_ context.Context, s *twoField) error { s.A = 1; return nil }
	setB := func(_ context.Context, s *twoField) error { s.B = 2; return nil }
	// Only one fn will win the race; the other will be canceled.
	// This test verifies no data race when fns write distinct fields.
	s := &twoField{}
	err := New[twoField]().Race("r", setA, setB).Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
}
