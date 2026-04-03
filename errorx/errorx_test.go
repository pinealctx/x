package errorx_test

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/pinealctx/x/errorx"
)

// Test code types for cross-domain testing.
type UserCode int
type ServiceCode int

const (
	UserNotFound     UserCode = 1
	UserUnauthorized UserCode = 2
)

const (
	ServiceUnavailable ServiceCode = 100
	ServiceTimeout     ServiceCode = 101
)

// --- Leaf errors: New / Newf ---

func TestNew_LeafError(t *testing.T) {
	err := errorx.New(UserNotFound, "user not found")
	if err.Code != UserNotFound {
		t.Errorf("Code = %d, want %d", err.Code, UserNotFound)
	}
	if err.Message != "user not found" {
		t.Errorf("Message = %q, want %q", err.Message, "user not found")
	}
	if err.Unwrap() != nil {
		t.Error("leaf Error should have nil Cause")
	}
}

func TestNewf_LeafError(t *testing.T) {
	err := errorx.Newf(UserNotFound, "user %s not found", "alice")
	if err.Code != UserNotFound {
		t.Errorf("Code = %d, want %d", err.Code, UserNotFound)
	}
	if err.Message != "user alice not found" {
		t.Errorf("Message = %q, want %q", err.Message, "user alice not found")
	}
	if err.Unwrap() != nil {
		t.Error("leaf Error should have nil Cause")
	}
}

// --- Wrapped errors: Wrap / Wrapf ---

func TestWrap_WrappedError(t *testing.T) {
	cause := io.EOF
	err := errorx.Wrap(cause, ServiceUnavailable, "service down")

	if err.Code != ServiceUnavailable {
		t.Errorf("Code = %d, want %d", err.Code, ServiceUnavailable)
	}
	if err.Message != "service down" {
		t.Errorf("Message = %q, want %q", err.Message, "service down")
	}
	if err.Unwrap() != cause {
		t.Error("Unwrap() should return the original cause")
	}
}

func TestWrapf_WrappedError(t *testing.T) {
	cause := io.EOF
	err := errorx.Wrapf(cause, ServiceUnavailable, "node %d down", 3)

	if err.Message != "node 3 down" {
		t.Errorf("Message = %q, want %q", err.Message, "node 3 down")
	}
	if err.Unwrap() != cause {
		t.Error("Unwrap() should return the original cause")
	}
}

func TestWrap_NilCause(t *testing.T) {
	err := errorx.Wrap(nil, ServiceUnavailable, "msg")
	if err != nil {
		t.Error("Wrap(nil, ...) should return nil")
	}
}

func TestWrapf_NilCause(t *testing.T) {
	err := errorx.Wrapf(nil, ServiceUnavailable, "msg %d", 1)
	if err != nil {
		t.Error("Wrapf(nil, ...) should return nil")
	}
}

// --- Error() format ---

func TestError_ReturnsMessageOnly(t *testing.T) {
	leaf := errorx.New(UserNotFound, "user not found")
	if leaf.Error() != "user not found" {
		t.Errorf("Error() = %q, want %q", leaf.Error(), "user not found")
	}

	wrapped := errorx.Wrap(io.EOF, ServiceUnavailable, "service down")
	if wrapped.Error() != "service down" {
		t.Errorf("Error() = %q, want %q", wrapped.Error(), "service down")
	}
}

// --- errors.Is / errors.As compatibility ---

func TestErrorsIs_Compatible(t *testing.T) {
	cause := io.EOF
	wrapped := errorx.Wrap(cause, ServiceUnavailable, "handler")
	if !errors.Is(wrapped, io.EOF) {
		t.Error("errors.Is should find io.EOF through Wrap chain")
	}
}

func TestErrorsAs_Compatible(t *testing.T) {
	inner := errorx.New(UserNotFound, "not found")
	outer := errorx.Wrap(inner, ServiceUnavailable, "handler")

	var target *errorx.Error[UserCode]
	if !errors.As(outer, &target) {
		t.Error("errors.As should find *Error[UserCode] through chain")
	}
	if target.Code != UserNotFound {
		t.Errorf("target.Code = %d, want %d", target.Code, UserNotFound)
	}
}

// --- IsCode ---

func TestIsCode_OutermostMatch(t *testing.T) {
	inner := errorx.New(UserNotFound, "inner")
	outer := errorx.Wrap(inner, UserUnauthorized, "outer")

	// Outermost is UserUnauthorized
	if !errorx.IsCode(outer, UserUnauthorized) {
		t.Error("IsCode should match outermost code")
	}
	// Inner is not the outermost
	if errorx.IsCode(outer, UserNotFound) {
		t.Error("IsCode should not match non-outermost code of same type")
	}
}

func TestIsCode_CrossDomainPenetration(t *testing.T) {
	leaf := errorx.New(UserNotFound, "user not found")          // *Error[UserCode]
	mid := fmt.Errorf("repo: %w", leaf)                         // *fmt.wrapError
	root := errorx.Wrap(mid, ServiceUnavailable, "unavailable") // *Error[ServiceCode]

	// IsCode[UserCode] should penetrate ServiceCode and fmt.Errorf to find UserCode
	if !errorx.IsCode(root, UserNotFound) {
		t.Error("IsCode should penetrate cross-domain nodes to find target code")
	}
}

func TestIsCode_NoMatch(t *testing.T) {
	err := errorx.New(UserNotFound, "not found")
	if errorx.IsCode(err, UserUnauthorized) {
		t.Error("IsCode should return false for non-matching code")
	}
}

func TestIsCode_NilError(t *testing.T) {
	if errorx.IsCode(nil, UserNotFound) {
		t.Error("IsCode(nil) should return false")
	}
}

func TestIsCode_NonErrorxType(t *testing.T) {
	if errorx.IsCode(io.EOF, UserNotFound) {
		t.Error("IsCode on non-Error type should return false")
	}
}

// --- ContainsCode ---

func TestContainsCode_FullChainSearch(t *testing.T) {
	inner := errorx.New(UserNotFound, "inner")
	outer := errorx.Wrap(inner, UserUnauthorized, "outer")

	// Both codes should be found anywhere in the chain
	if !errorx.ContainsCode(outer, UserUnauthorized) {
		t.Error("ContainsCode should find outermost code")
	}
	if !errorx.ContainsCode(outer, UserNotFound) {
		t.Error("ContainsCode should find inner code")
	}
}

func TestContainsCode_CrossDomainPenetration(t *testing.T) {
	leaf := errorx.New(UserNotFound, "user not found")          // *Error[UserCode]
	mid := fmt.Errorf("repo: %w", leaf)                         // *fmt.wrapError
	root := errorx.Wrap(mid, ServiceUnavailable, "unavailable") // *Error[ServiceCode]

	if !errorx.ContainsCode(root, UserNotFound) {
		t.Error("ContainsCode should penetrate cross-domain nodes to find target code")
	}
}

func TestContainsCode_NoMatch(t *testing.T) {
	err := errorx.New(UserNotFound, "not found")
	if errorx.ContainsCode(err, UserUnauthorized) {
		t.Error("ContainsCode should return false for non-matching code")
	}
}

func TestContainsCode_NilError(t *testing.T) {
	if errorx.ContainsCode(nil, UserNotFound) {
		t.Error("ContainsCode(nil) should return false")
	}
}

// --- Empty chain (leaf) safety ---

func TestIsCode_LeafError(t *testing.T) {
	err := errorx.New(UserNotFound, "not found")
	if !errorx.IsCode(err, UserNotFound) {
		t.Error("IsCode should match leaf error code")
	}
}

func TestContainsCode_LeafError(t *testing.T) {
	err := errorx.New(UserNotFound, "not found")
	if !errorx.ContainsCode(err, UserNotFound) {
		t.Error("ContainsCode should match leaf error code")
	}
}

// --- Cross-domain failure paths ---

func TestContainsCode_CrossDomainCodeMismatch(t *testing.T) {
	leaf := errorx.New(UserNotFound, "user not found")
	mid := fmt.Errorf("repo: %w", leaf)
	root := errorx.Wrap(mid, ServiceUnavailable, "unavailable")

	// Penetrates to UserCode but code doesn't match
	if errorx.ContainsCode(root, UserUnauthorized) {
		t.Error("ContainsCode should return false when code doesn't match in target domain")
	}
}

func TestContainsCode_CrossDomainOuterMatch(t *testing.T) {
	leaf := errorx.New(UserNotFound, "user not found")
	mid := fmt.Errorf("repo: %w", leaf)
	root := errorx.Wrap(mid, ServiceUnavailable, "unavailable")

	// ServiceCode is at the outer layer, should be found directly
	if !errorx.ContainsCode(root, ServiceUnavailable) {
		t.Error("ContainsCode should find ServiceCode at outer layer")
	}
}

// --- Code zero value ---

func TestIsCode_ZeroCode(t *testing.T) {
	err := errorx.New(UserCode(0), "zero code")
	if !errorx.IsCode(err, UserCode(0)) {
		t.Error("IsCode should match zero code value")
	}
	if errorx.IsCode(err, UserNotFound) {
		t.Error("IsCode should not match non-zero code")
	}
}

func TestContainsCode_ZeroCode(t *testing.T) {
	err := errorx.New(UserCode(0), "zero code")
	if !errorx.ContainsCode(err, UserCode(0)) {
		t.Error("ContainsCode should match zero code value")
	}
}

// --- errors.As cross-domain rejection ---

func TestErrorsAs_CrossDomainRejection(t *testing.T) {
	// Chain only contains ServiceCode, not UserCode
	err := errorx.New(ServiceUnavailable, "unavailable")

	var target *errorx.Error[UserCode]
	if errors.As(err, &target) {
		t.Error("errors.As should not match *Error[UserCode] when chain only has ServiceCode")
	}
}

// --- Sentinel + Error mixed chain ---

func TestWrap_SentinelCause(t *testing.T) {
	sentinel := ErrorA("connection refused")
	err := errorx.Wrap(sentinel, ServiceUnavailable, "service failed")

	if err.Unwrap() != sentinel {
		t.Error("Unwrap should return the sentinel error")
	}
	if !errors.Is(err, sentinel) {
		t.Error("errors.Is should find sentinel through Error wrap")
	}
}
