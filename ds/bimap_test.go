package ds

import "testing"

func TestBiMap_BasicOperations(t *testing.T) {
	m := NewBiMap[string, int]()
	m.Set("alice", 1)
	m.Set("bob", 2)

	v, ok := m.GetByKey("alice")
	if !ok || v != 1 {
		t.Fatalf("GetByKey(alice) = %d, %v", v, ok)
	}
	k, ok := m.GetByValue(2)
	if !ok || k != "bob" {
		t.Fatalf("GetByValue(2) = %q, %v", k, ok)
	}
	if m.Len() != 2 {
		t.Fatalf("Len = %d, want 2", m.Len())
	}
	checkBiMapInvariant(t, m)
}

func TestBiMap_SetOverwritesOldKey(t *testing.T) {
	m := NewBiMap[string, int]()
	m.Set("alice", 1)
	m.Set("bob", 2)

	// overwrite alice's value: 1 → 10
	m.Set("alice", 10)

	v, ok := m.GetByKey("alice")
	if !ok || v != 10 {
		t.Fatalf("GetByKey(alice) = %d, want 10", v)
	}
	// old value 1 should be gone from inverse
	_, ok = m.GetByValue(1)
	if ok {
		t.Fatal("GetByValue(1) should return false after overwrite")
	}
	// len should remain 2
	if m.Len() != 2 {
		t.Fatalf("Len = %d, want 2", m.Len())
	}
	checkBiMapInvariant(t, m)
}

func TestBiMap_SetOverwritesOldValue(t *testing.T) {
	m := NewBiMap[string, int]()
	m.Set("alice", 1)
	m.Set("bob", 2)

	// value 2 is now taken by bob; set charlie→2 should evict bob
	m.Set("charlie", 2)

	v, ok := m.GetByKey("charlie")
	if !ok || v != 2 {
		t.Fatalf("GetByKey(charlie) = %d, want 2", v)
	}
	k, ok := m.GetByValue(2)
	if !ok || k != "charlie" {
		t.Fatalf("GetByValue(2) = %q, want charlie", k)
	}
	// bob should be gone
	_, ok = m.GetByKey("bob")
	if ok {
		t.Fatal("GetByKey(bob) should return false")
	}
	checkBiMapInvariant(t, m)
}

func TestBiMap_DeleteByKey(t *testing.T) {
	m := NewBiMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)

	if !m.DeleteByKey("a") {
		t.Fatal("DeleteByKey(a) should return true")
	}
	if m.DeleteByKey("a") {
		t.Fatal("DeleteByKey(a) again should return false")
	}
	_, ok := m.GetByValue(1)
	if ok {
		t.Fatal("GetByValue(1) should return false after DeleteByKey(a)")
	}
	if m.Len() != 1 {
		t.Fatalf("Len = %d, want 1", m.Len())
	}
	checkBiMapInvariant(t, m)
}

func TestBiMap_DeleteByValue(t *testing.T) {
	m := NewBiMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)

	if !m.DeleteByValue(1) {
		t.Fatal("DeleteByValue(1) should return true")
	}
	if m.DeleteByValue(1) {
		t.Fatal("DeleteByValue(1) again should return false")
	}
	_, ok := m.GetByKey("a")
	if ok {
		t.Fatal("GetByKey(a) should return false after DeleteByValue(1)")
	}
	checkBiMapInvariant(t, m)
}

func TestBiMap_EmptyOperations(t *testing.T) {
	m := NewBiMap[string, int]()
	_, ok := m.GetByKey("x")
	if ok {
		t.Fatal("GetByKey on empty should return false")
	}
	_, ok = m.GetByValue(1)
	if ok {
		t.Fatal("GetByValue on empty should return false")
	}
	if m.DeleteByKey("x") {
		t.Fatal("DeleteByKey on empty should return false")
	}
	if m.DeleteByValue(1) {
		t.Fatal("DeleteByValue on empty should return false")
	}
	if m.Len() != 0 {
		t.Fatal("Len on empty should be 0")
	}
}

func TestBiMap_AllKeysValues(t *testing.T) {
	m := NewBiMap[int, string]()
	m.Set(1, "a")
	m.Set(2, "b")
	m.Set(3, "c")

	// collect All
	all := map[int]string{}
	for k, v := range m.All() {
		all[k] = v
	}
	if len(all) != 3 {
		t.Fatalf("All yielded %d pairs, want 3", len(all))
	}
	for k, v := range all {
		if got, _ := m.GetByKey(k); got != v {
			t.Fatalf("All pair (%d,%q) doesn't match GetByKey", k, v)
		}
	}

	keys := m.Keys()
	if len(keys) != 3 {
		t.Fatalf("Keys len = %d, want 3", len(keys))
	}
	vals := m.Values()
	if len(vals) != 3 {
		t.Fatalf("Values len = %d, want 3", len(vals))
	}
}

func TestBiMap_GenericInstantiation(t *testing.T) {
	// string↔int
	m1 := NewBiMap[string, int]()
	m1.Set("x", 42)
	k, _ := m1.GetByValue(42)
	if k != "x" {
		t.Fatalf("string↔int: GetByValue(42) = %q, want x", k)
	}

	// int↔string
	m2 := NewBiMap[int, string]()
	m2.Set(42, "x")
	v, _ := m2.GetByKey(42)
	if v != "x" {
		t.Fatalf("int↔string: GetByKey(42) = %q, want x", v)
	}
}

func TestBiMap_SetSameKeySameValue(t *testing.T) {
	m := NewBiMap[string, int]()
	m.Set("a", 1)
	m.Set("a", 1) // idempotent
	if m.Len() != 1 {
		t.Fatalf("Len after idempotent Set = %d, want 1", m.Len())
	}
}

func TestBiMap_SameType_KEqualsV(t *testing.T) {
	// K and V are both int — key and value domains overlap
	m := NewBiMap[int, int]()
	m.Set(1, 2)
	m.Set(3, 1) // value 1 is now taken; key 1's old value 2 is evicted from inverse

	// key 1 should still map to 2
	v, ok := m.GetByKey(1)
	if !ok || v != 2 {
		t.Fatalf("GetByKey(1) = %d, want 2", v)
	}
	// value 1 should map to key 3
	k, ok := m.GetByValue(1)
	if !ok || k != 3 {
		t.Fatalf("GetByValue(1) = %d, want 3", k)
	}
	// key 3 should map to value 1
	v, ok = m.GetByKey(3)
	if !ok || v != 1 {
		t.Fatalf("GetByKey(3) = %d, want 1", v)
	}
	checkBiMapInvariant(t, m)
}

func TestBiMap_AllBreak(t *testing.T) {
	m := NewBiMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)
	var count int
	for range m.All() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Fatalf("All break count = %d, want 2", count)
	}
}

// checkBiMapInvariant verifies forward and inverse maps stay consistent.
func checkBiMapInvariant[K comparable, V comparable](t *testing.T, m *BiMap[K, V]) {
	t.Helper()
	if len(m.forward) != len(m.inverse) {
		t.Fatalf("forward len %d != inverse len %d", len(m.forward), len(m.inverse))
	}
	for k, v := range m.forward {
		ik, ok := m.inverse[v]
		if !ok || ik != k {
			t.Fatalf("inverse[%v] = %v, want %v", v, ik, k)
		}
	}
}

func TestBiMap_Clear(t *testing.T) {
	m := NewBiMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Clear()
	if m.Len() != 0 {
		t.Fatalf("after Clear: Len = %d, want 0", m.Len())
	}
	// can still use after clear
	m.Set("c", 3)
	if m.Len() != 1 {
		t.Fatalf("after Clear+Set: Len = %d, want 1", m.Len())
	}
	checkBiMapInvariant(t, m)
}

func TestBiMap_ConcurrentRead(_ *testing.T) {
	m := NewBiMap[int, string]()
	for i := range 100 {
		m.Set(i, "v")
	}
	done := make(chan struct{})
	for range 4 {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := range 100 {
				m.GetByKey(i)
				m.GetByValue("v")
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

func TestBiMap_SystematicConsistency(t *testing.T) {
	m := NewBiMap[int, string]()
	// series of mutations
	m.Set(1, "a")
	checkBiMapInvariant(t, m)
	m.Set(2, "b")
	checkBiMapInvariant(t, m)
	m.Set(1, "c") // overwrite key 1's value
	checkBiMapInvariant(t, m)
	m.Set(3, "b") // overwrite value "b"'s key (evicts key 2)
	checkBiMapInvariant(t, m)
	m.DeleteByKey(1)
	checkBiMapInvariant(t, m)
	m.DeleteByValue("b")
	checkBiMapInvariant(t, m)

	// remaining: nothing (all evicted/deleted)
	if m.Len() != 0 {
		t.Fatalf("expected empty after all deletions, got Len=%d", m.Len())
	}
}
