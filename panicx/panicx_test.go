package panicx_test

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/pinealctx/x/panicx"
)

// mustCapture runs fn inside a standard defer/recover block and returns the PanicError.
func mustCapture(fn func()) *panicx.PanicError {
	var pe *panicx.PanicError
	func() {
		defer func() {
			if r := recover(); r != nil {
				pe = panicx.NewPanicError(r)
			}
		}()
		fn()
	}()
	return pe
}

// hasFrame reports whether any frame in stack contains substr.
func hasFrame(stack []string, substr string) bool {
	for _, f := range stack {
		if strings.Contains(f, substr) {
			return true
		}
	}
	return false
}

// --- errors.Is / errors.As ---

func TestIs(t *testing.T) {
	pe := mustCapture(func() { panic("test") })
	if !errors.Is(pe, panicx.ErrPanic) {
		t.Fatal("expected errors.Is to return true")
	}
}

func TestAs(t *testing.T) {
	pe := mustCapture(func() { panic("hello") })
	var got *panicx.PanicError
	if !errors.As(pe, &got) {
		t.Fatal("expected errors.As to succeed")
	}
	if got.Value != "hello" {
		t.Fatalf("Value = %v, want hello", got.Value)
	}
}

// --- Error() format ---

func TestError_Format(t *testing.T) {
	pe := mustCapture(func() { panic("boom") })
	if !strings.Contains(pe.Error(), "boom") {
		t.Fatalf("Error() = %q, want to contain panic value", pe.Error())
	}
	if !strings.Contains(pe.Error(), "panic recovered") {
		t.Fatalf("Error() = %q, want prefix 'panic recovered'", pe.Error())
	}
}

// --- Stack() per panic type ---

func triggerString() { panic("msg") }
func triggerError()  { panic(errors.New("err")) }
func triggerInt()    { panic(42) }
func triggerNil()    { panic(nil) }
func triggerDivZero() {
	a, b := 1, 0
	_ = a / b
}
func triggerNilPointer() { _ = *(*int)(nil) }
func triggerNilMap() {
	var m map[string]int
	m["k"] = 1 //nolint:staticcheck // intentional nil map write to trigger panic
}
func triggerOutOfBounds() {
	s := []int{1}
	_ = s[5] //nolint:gosec // intentional out-of-bounds to trigger panic
}

func deepC() { panic("deep") }
func deepB() { deepC() }
func deepA() { deepB() }

func TestStack_ExplicitString(t *testing.T) {
	pe := mustCapture(triggerString)
	stack := pe.Stack()
	if len(stack) == 0 {
		t.Fatal("expected non-empty stack")
	}
	if !hasFrame(stack, "triggerString") {
		t.Fatalf("stack missing triggerString:\n%s", strings.Join(stack, "\n"))
	}
}

func TestStack_ExplicitError(t *testing.T) {
	pe := mustCapture(triggerError)
	if _, ok := pe.Value.(error); !ok {
		t.Fatalf("Value type = %T, want error", pe.Value)
	}
	if !hasFrame(pe.Stack(), "triggerError") {
		t.Fatalf("stack missing triggerError:\n%s", strings.Join(pe.Stack(), "\n"))
	}
}

func TestStack_ExplicitInt(t *testing.T) {
	pe := mustCapture(triggerInt)
	if pe.Value != 42 {
		t.Fatalf("Value = %v, want 42", pe.Value)
	}
	if !hasFrame(pe.Stack(), "triggerInt") {
		t.Fatalf("stack missing triggerInt:\n%s", strings.Join(pe.Stack(), "\n"))
	}
}

func TestStack_ExplicitNil(t *testing.T) {
	pe := mustCapture(triggerNil)
	// Go 1.21+: panic(nil) wraps into *runtime.PanicNilError
	if pe.Value == nil {
		t.Fatal("expected non-nil Value for panic(nil) on Go 1.21+")
	}
	if len(pe.Stack()) == 0 {
		t.Fatal("expected non-empty stack")
	}
}

func TestStack_DivideByZero(t *testing.T) {
	pe := mustCapture(triggerDivZero)
	stack := pe.Stack()
	if !hasFrame(stack, "panicdivide") && !hasFrame(stack, "triggerDivZero") {
		t.Fatalf("stack missing expected frames:\n%s", strings.Join(stack, "\n"))
	}
}

func TestStack_NilPointer(t *testing.T) {
	pe := mustCapture(triggerNilPointer)
	stack := pe.Stack()
	if !hasFrame(stack, "sigpanic") && !hasFrame(stack, "triggerNilPointer") {
		t.Fatalf("stack missing expected frames:\n%s", strings.Join(stack, "\n"))
	}
}

func TestStack_NilMapWrite(t *testing.T) {
	pe := mustCapture(triggerNilMap)
	if !hasFrame(pe.Stack(), "triggerNilMap") {
		t.Fatalf("stack missing triggerNilMap:\n%s", strings.Join(pe.Stack(), "\n"))
	}
}

func TestStack_OutOfBounds(t *testing.T) {
	pe := mustCapture(triggerOutOfBounds)
	if !hasFrame(pe.Stack(), "triggerOutOfBounds") {
		t.Fatalf("stack missing triggerOutOfBounds:\n%s", strings.Join(pe.Stack(), "\n"))
	}
}

func TestStack_DeepNested(t *testing.T) {
	pe := mustCapture(deepA)
	stack := pe.Stack()
	for _, fn := range []string{"deepA", "deepB", "deepC"} {
		if !hasFrame(stack, fn) {
			t.Fatalf("stack missing %s:\n%s", fn, strings.Join(stack, "\n"))
		}
	}
	// verify order: deepC appears before deepB before deepA
	idxC, idxB, idxA := -1, -1, -1
	for i, f := range stack {
		if strings.Contains(f, "deepC") {
			idxC = i
		} else if strings.Contains(f, "deepB") {
			idxB = i
		} else if strings.Contains(f, "deepA") {
			idxA = i
		}
	}
	if idxC >= idxB || idxB >= idxA {
		t.Fatalf("unexpected frame order: deepC=%d deepB=%d deepA=%d\n%s", idxC, idxB, idxA, strings.Join(stack, "\n"))
	}
}

// --- Stack() behavior ---

func TestStack_CachedResult(t *testing.T) {
	pe := mustCapture(func() { panic("cache") })
	s1 := pe.Stack()
	s2 := pe.Stack()
	if len(s1) != len(s2) {
		t.Fatal("Stack() returned different lengths on repeated calls")
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			t.Fatalf("Stack()[%d] differs between calls", i)
		}
	}
}

func TestStack_ConcurrentSafe(_ *testing.T) {
	pe := mustCapture(func() { panic("concurrent") })
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pe.Stack()
		}()
	}
	wg.Wait()
}

// --- NewPanicErrorSkip ---

func TestNewPanicErrorSkip(t *testing.T) {
	// skip=0: frame[0] is the defer func (direct caller of NewPanicError)
	// skip=1: frame[0] should be one level up
	var pe0, pe1 *panicx.PanicError
	func() {
		defer func() {
			if r := recover(); r != nil {
				pe0 = panicx.NewPanicErrorSkip(r, 0)
			}
		}()
		panic("skip-test")
	}()
	func() {
		defer func() {
			if r := recover(); r != nil {
				pe1 = panicx.NewPanicErrorSkip(r, 1)
			}
		}()
		panic("skip-test")
	}()

	s0 := pe0.Stack()
	s1 := pe1.Stack()
	if len(s0) == 0 || len(s1) == 0 {
		t.Fatal("expected non-empty stacks")
	}
	// skip=1 should have one fewer frame at the top
	if len(s1) >= len(s0) {
		t.Fatalf("skip=1 stack len=%d should be less than skip=0 stack len=%d", len(s1), len(s0))
	}
}
