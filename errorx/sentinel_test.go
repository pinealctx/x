package errorx_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/pinealctx/x/errorx"
)

// Domain tags for testing domain isolation.
type domainA struct{}
type domainB struct{}

type ErrorA = errorx.Sentinel[domainA]
type ErrorB = errorx.Sentinel[domainB]

func TestSentinel_Error(t *testing.T) {
	s := errorx.NewSentinel[domainA]("connection refused")
	if got := s.Error(); got != "connection refused" {
		t.Errorf("Error() = %q, want %q", got, "connection refused")
	}
}

func TestSentinel_NewSentinelf(t *testing.T) {
	s := errorx.NewSentinelf[domainA]("item %d not found: %s", 42, "user")
	if got := s.Error(); got != "item 42 not found: user" {
		t.Errorf("Error() = %q, want %q", got, "item 42 not found: user")
	}
}

func TestSentinel_SameDomainMatch(t *testing.T) {
	err := ErrorA("timeout")
	var target ErrorA
	if !errors.As(err, &target) {
		t.Error("errors.As should match Sentinel of same domain")
	}
	if target != "timeout" {
		t.Errorf("target = %q, want %q", target, "timeout")
	}
}

func TestSentinel_CrossDomainNoMatch(t *testing.T) {
	err := ErrorA("timeout")
	var target ErrorB
	if errors.As(err, &target) {
		t.Error("errors.As should not match Sentinel of different domain")
	}
}

func TestSentinel_CrossDomainDifferentValues(t *testing.T) {
	// Same message, different domain — must not match
	a := ErrorA("timeout")
	b := ErrorB("timeout")
	if errors.Is(a, b) {
		t.Error("errors.Is should not match Sentinel of different domain with same message")
	}
}

func TestSentinel_WrappedByFmtErrorf(t *testing.T) {
	sentinel := ErrorA("connection refused")
	wrapped := fmt.Errorf("dial failed: %w", sentinel)

	var target ErrorA
	if !errors.As(wrapped, &target) {
		t.Error("errors.As should find Sentinel through fmt.Errorf wrapper")
	}
	if target != "connection refused" {
		t.Errorf("target = %q, want %q", target, "connection refused")
	}

	// errors.Is should also work through wrapper
	if !errors.Is(wrapped, sentinel) {
		t.Error("errors.Is should find Sentinel through fmt.Errorf wrapper")
	}
}
