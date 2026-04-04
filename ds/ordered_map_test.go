package ds

import (
	"slices"
	"testing"
)

// --- OrderedMap tests ---

func TestOrderedMap_SetGetHasDelete(t *testing.T) {
	m := NewOrderedMap[string, int]()
	if m.Len() != 0 {
		t.Fatalf("empty: Len() = %d, want 0", m.Len())
	}

	// new insert returns true
	if !m.Set("a", 1) {
		t.Fatal("Set new key should return true")
	}
	if !m.Set("b", 2) {
		t.Fatal("Set new key should return true")
	}
	if m.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", m.Len())
	}

	// Get
	if v, ok := m.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %d, %v; want 1, true", v, ok)
	}
	if v, ok := m.Get("b"); !ok || v != 2 {
		t.Fatalf("Get(b) = %d, %v; want 2, true", v, ok)
	}
	if _, ok := m.Get("z"); ok {
		t.Fatal("Get(z) should return false")
	}

	// Has
	if !m.Has("a") {
		t.Fatal("Has(a) should be true")
	}
	if m.Has("z") {
		t.Fatal("Has(z) should be false")
	}

	// Delete
	if !m.Delete("a") {
		t.Fatal("Delete(a) should return true")
	}
	if m.Has("a") {
		t.Fatal("Has(a) after delete should be false")
	}
	if m.Delete("a") {
		t.Fatal("Delete(a) again should return false")
	}
}

func TestOrderedMap_SetExistingKeyUpdatesValue(t *testing.T) {
	m := NewOrderedMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)

	// update existing key
	if m.Set("a", 10) {
		t.Fatal("Set existing key should return false")
	}
	if v, ok := m.Get("a"); !ok || v != 10 {
		t.Fatalf("Get(a) after update = %d, want 10", v)
	}

	// position unchanged: a should still be first
	keys := m.Keys()
	if !slices.Equal(keys, []string{"a", "b"}) {
		t.Fatalf("order after update = %v, want [a b]", keys)
	}
}

func TestOrderedMap_InsertionOrder(t *testing.T) {
	m := NewOrderedMap[int, string]()
	for i, v := range []string{"a", "b", "c", "d", "e"} {
		m.Set(i, v)
	}

	// All forward
	var forward []string
	for _, v := range m.All() {
		forward = append(forward, v)
	}
	if want := []string{"a", "b", "c", "d", "e"}; !slices.Equal(forward, want) {
		t.Fatalf("All() = %v, want %v", forward, want)
	}

	// Backward
	var backward []string
	for _, v := range m.Backward() {
		backward = append(backward, v)
	}
	if want := []string{"e", "d", "c", "b", "a"}; !slices.Equal(backward, want) {
		t.Fatalf("Backward() = %v, want %v", backward, want)
	}
}

func TestOrderedMap_DeleteMaintainsIntegrity(t *testing.T) {
	m := NewOrderedMap[int, string]()
	m.Set(1, "a")
	m.Set(2, "b")
	m.Set(3, "c")

	m.Delete(2)
	want := []int{1, 3}
	keys := m.Keys()
	if !slices.Equal(keys, want) {
		t.Fatalf("after delete(2): keys = %v, want %v", keys, want)
	}

	// delete front
	m.Delete(1)
	keys = m.Keys()
	if !slices.Equal(keys, []int{3}) {
		t.Fatalf("after delete(1): keys = %v, want [3]", keys)
	}

	// delete back
	m.Delete(3)
	keys = m.Keys()
	if len(keys) != 0 {
		t.Fatalf("after all deletes: keys = %v, want []", keys)
	}
	if m.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", m.Len())
	}
}

func TestOrderedMap_EmptyOperations(t *testing.T) {
	m := NewOrderedMap[string, int]()

	_, ok := m.Get("x")
	if ok {
		t.Fatal("Get on empty should return false")
	}
	if m.Has("x") {
		t.Fatal("Has on empty should return false")
	}
	if m.Delete("x") {
		t.Fatal("Delete on empty should return false")
	}
	if m.Len() != 0 {
		t.Fatal("Len on empty should be 0")
	}

	// All should yield nothing
	count := 0
	for range m.All() {
		count++
	}
	if count != 0 {
		t.Fatal("All on empty should yield nothing")
	}

	// Backward should yield nothing
	for range m.Backward() {
		count++
	}
	if count != 0 {
		t.Fatal("Backward on empty should yield nothing")
	}
}

func TestOrderedMap_KeysValuesMatchAll(t *testing.T) {
	m := NewOrderedMap[string, int]()
	m.Set("x", 10)
	m.Set("y", 20)
	m.Set("z", 30)

	keys := m.Keys()
	vals := m.Values()

	var allKeys []string
	var allVals []int
	for k, v := range m.All() {
		allKeys = append(allKeys, k)
		allVals = append(allVals, v)
	}

	if !slices.Equal(keys, allKeys) {
		t.Fatalf("Keys() = %v, All keys = %v", keys, allKeys)
	}
	if !slices.Equal(vals, allVals) {
		t.Fatalf("Values() = %v, All vals = %v", vals, allVals)
	}
}

func TestOrderedMap_CapacityOne(t *testing.T) {
	m := NewOrderedMapWithCapacity[int, string](1)
	m.Set(1, "one")

	if v, ok := m.Get(1); !ok || v != "one" {
		t.Fatalf("Get(1) = %q, %v", v, ok)
	}
	m.Delete(1)
	if m.Len() != 0 {
		t.Fatalf("after delete: Len() = %d", m.Len())
	}
	// re-insert after delete
	m.Set(2, "two")
	if m.Len() != 1 {
		t.Fatalf("after re-insert: Len() = %d", m.Len())
	}
}

func TestOrderedMap_GenericInstantiation(t *testing.T) {
	// int -> string
	m1 := NewOrderedMap[int, string]()
	m1.Set(1, "a")
	if v, ok := m1.Get(1); !ok || v != "a" {
		t.Fatalf("int->string: %q, %v", v, ok)
	}

	// string -> int
	m2 := NewOrderedMap[string, int]()
	m2.Set("hello", 42)
	if v, ok := m2.Get("hello"); !ok || v != 42 {
		t.Fatalf("string->int: %d, %v", v, ok)
	}

	// struct key
	type pair struct{ A, B int }
	m3 := NewOrderedMap[pair, float64]()
	m3.Set(pair{1, 2}, 3.14)
	if v, ok := m3.Get(pair{1, 2}); !ok || v != 3.14 {
		t.Fatalf("struct key: %v, %v", v, ok)
	}
}

func TestOrderedMap_ConcurrentRead(_ *testing.T) {
	m := NewOrderedMap[int, int]()
	for i := range 100 {
		m.Set(i, i*10)
	}

	// multiple goroutines reading simultaneously should not race
	done := make(chan struct{})
	for range 4 {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := range 100 {
				m.Get(i)
				m.Has(i)
			}
			for range m.All() {
				break
			}
		}()
	}
	for range 4 {
		<-done
	}
}

func TestOrderedMap_AllBreak(t *testing.T) {
	m := NewOrderedMap[int, int]()
	for i := range 10 {
		m.Set(i, i)
	}
	var collected []int
	for k := range m.All() {
		collected = append(collected, k)
		if k == 3 {
			break
		}
	}
	if !slices.Equal(collected, []int{0, 1, 2, 3}) {
		t.Fatalf("All break = %v, want [0 1 2 3]", collected)
	}
}

func TestOrderedMap_BackwardBreak(t *testing.T) {
	m := NewOrderedMap[int, int]()
	for i := range 10 {
		m.Set(i, i)
	}
	var collected []int
	for k := range m.Backward() {
		collected = append(collected, k)
		if k == 7 {
			break
		}
	}
	if !slices.Equal(collected, []int{9, 8, 7}) {
		t.Fatalf("Backward break = %v, want [9 8 7]", collected)
	}
}

func TestOrderedMap_Clear(t *testing.T) {
	m := NewOrderedMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)
	m.Clear()
	if m.Len() != 0 {
		t.Fatalf("after Clear: Len = %d, want 0", m.Len())
	}
	// can still use after clear
	m.Set("d", 4)
	if m.Len() != 1 {
		t.Fatalf("after Clear+Set: Len = %d, want 1", m.Len())
	}
	v, ok := m.Get("d")
	if !ok || v != 4 {
		t.Fatalf("after Clear+Set: Get(d) = %d, want 4", v)
	}
}

func TestOrderedMap_DeleteMiddleIntegrity(t *testing.T) {
	m := NewOrderedMap[int, string]()
	for i := range 5 {
		m.Set(i, "v")
	}
	// delete middle element
	m.Delete(2)

	// verify forward iteration skips deleted
	var forward []int
	for k := range m.All() {
		forward = append(forward, k)
	}
	if !slices.Equal(forward, []int{0, 1, 3, 4}) {
		t.Fatalf("forward after delete(2) = %v", forward)
	}

	// verify backward iteration
	var backward []int
	for k := range m.Backward() {
		backward = append(backward, k)
	}
	if !slices.Equal(backward, []int{4, 3, 1, 0}) {
		t.Fatalf("backward after delete(2) = %v", backward)
	}
}

func TestOrderedMap_NegativeCapacityPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for negative capacity")
		}
	}()
	NewOrderedMapWithCapacity[int, int](-1)
}
