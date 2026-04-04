package ds

import (
	"slices"
	"testing"
)

func TestStack_PushPopOrder(t *testing.T) {
	s := NewStack[int]()
	s.Push(1)
	s.Push(2)
	s.Push(3)

	v, ok := s.Pop()
	if !ok || v != 3 {
		t.Fatalf("Pop() = %d, %v; want 3, true", v, ok)
	}
	v, ok = s.Pop()
	if !ok || v != 2 {
		t.Fatalf("Pop() = %d, %v; want 2, true", v, ok)
	}
	v, ok = s.Pop()
	if !ok || v != 1 {
		t.Fatalf("Pop() = %d, %v; want 1, true", v, ok)
	}
	_, ok = s.Pop()
	if ok {
		t.Fatal("Pop on empty should return false")
	}
}

func TestStack_PeekNoRemove(t *testing.T) {
	s := NewStack[string]()
	s.Push("a")
	s.Push("b")

	v, ok := s.Peek()
	if !ok || v != "b" {
		t.Fatalf("Peek() = %q, %v; want b, true", v, ok)
	}
	if s.Len() != 2 {
		t.Fatalf("Len after Peek = %d, want 2", s.Len())
	}
}

func TestStack_EmptyOperations(t *testing.T) {
	s := NewStack[float64]()
	_, ok := s.Pop()
	if ok {
		t.Fatal("Pop on empty should return false")
	}
	_, ok = s.Peek()
	if ok {
		t.Fatal("Peek on empty should return false")
	}
	if s.Len() != 0 {
		t.Fatal("Len on empty should be 0")
	}
}

func TestStack_AllLIFOOrder(t *testing.T) {
	s := NewStack[int]()
	s.Push(1)
	s.Push(2)
	s.Push(3)

	var collected []int
	for v := range s.All() {
		collected = append(collected, v)
	}
	if !slices.Equal(collected, []int{3, 2, 1}) {
		t.Fatalf("All LIFO = %v, want [3 2 1]", collected)
	}
	// All should not modify stack
	if s.Len() != 3 {
		t.Fatalf("Len after All = %d, want 3", s.Len())
	}
}

func TestStack_WithCapacity(t *testing.T) {
	s := NewStackWithCapacity[int](100)
	if s.Len() != 0 {
		t.Fatal("empty stack Len != 0")
	}
	for i := range 50 {
		s.Push(i)
	}
	if s.Len() != 50 {
		t.Fatalf("Len = %d, want 50", s.Len())
	}
}

func TestStack_Clear(t *testing.T) {
	s := NewStack[int]()
	s.Push(1)
	s.Push(2)
	s.Clear()
	if s.Len() != 0 {
		t.Fatalf("after Clear: Len = %d, want 0", s.Len())
	}
	// can still use after clear
	s.Push(10)
	v, ok := s.Peek()
	if !ok || v != 10 {
		t.Fatalf("after Clear+Push: Peek = %d, want 10", v)
	}
}

func TestStack_NegativeCapacityPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for negative capacity")
		}
	}()
	NewStackWithCapacity[int](-1)
}
