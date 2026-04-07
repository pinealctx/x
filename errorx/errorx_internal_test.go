package errorx

import (
	"testing"
)

// TestContainsCode_MaxDepthExceeded verifies that ContainsCode stops traversal
// at maxContainsDepth and returns false when the target code is beyond the limit.
// The chain must consist of Error[testCode] nodes (not fmt.Errorf wrappers),
// because errors.As penetrates non-matching wrapper types in a single step.
func TestContainsCode_MaxDepthExceeded(t *testing.T) {
	type testCode int
	const (
		decoy  testCode = 2
		target testCode = 1
	)

	// Build a chain of maxContainsDepth Error[testCode] nodes with code=decoy,
	// then append the target at the end. ContainsCode exhausts its budget on
	// the decoy nodes and never reaches the target.
	var err error = New(target, "leaf")
	for range maxContainsDepth {
		err = Wrap(err, decoy, "decoy")
	}

	if ContainsCode(err, target) {
		t.Error("ContainsCode should return false when target is beyond maxContainsDepth")
	}
}

// TestContainsCode_ChainExhausted verifies that ContainsCode returns false
// when the chain is fully traversed without finding the target code.
func TestContainsCode_ChainExhausted(t *testing.T) {
	type testCode int
	const (
		decoy  testCode = 2
		target testCode = 1
	)

	// A short chain with only decoy nodes — err becomes nil after the last node.
	err := Wrap(New(decoy, "inner"), decoy, "outer")

	if ContainsCode(err, target) {
		t.Error("ContainsCode should return false when chain is exhausted without finding target")
	}
}

// TestContainsCode_CrossTypeBoundary verifies that ContainsCode returns false
// when the chain crosses into a different Code type (errors.As cannot match).
func TestContainsCode_CrossTypeBoundary(t *testing.T) {
	type codeA int
	type codeB int
	const (
		valA codeA = 1
		valB codeB = 1
	)

	// Chain: Error[codeA](decoy) → Error[codeB](valB)
	// ContainsCode[codeA] finds the codeA node (decoy, not target),
	// then steps to Cause which is *Error[codeB] — errors.As[codeA] fails.
	const decoyA codeA = 2
	inner := New(valB, "inner-b")
	outer := Wrap(inner, decoyA, "outer-a")

	if ContainsCode(outer, valA) {
		t.Error("ContainsCode should return false when chain crosses Code type boundary")
	}
}

// TestContainsCode_ExactlyAtMaxDepth verifies that ContainsCode finds the target code
// when it is at exactly maxContainsDepth-1 layers deep (just within the limit).
func TestContainsCode_ExactlyAtMaxDepth(t *testing.T) {
	type testCode int
	const (
		decoy  testCode = 2
		target testCode = 1
	)

	// Build a chain with maxContainsDepth-1 decoy nodes — just within limit.
	var err error = New(target, "leaf")
	for range maxContainsDepth - 1 {
		err = Wrap(err, decoy, "decoy")
	}

	if !ContainsCode(err, target) {
		t.Error("ContainsCode should find code at exactly maxContainsDepth-1 layers")
	}
}
