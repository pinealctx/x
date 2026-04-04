package ds

import (
	"testing"
)

func TestSet_BasicOperations(t *testing.T) {
	s := NewSet[int]()
	if s.Len() != 0 {
		t.Fatal("empty set Len != 0")
	}

	if !s.Add(1) {
		t.Fatal("Add new should return true")
	}
	if s.Add(1) {
		t.Fatal("Add duplicate should return false")
	}
	if !s.Contains(1) {
		t.Fatal("Contains(1) should be true")
	}
	if s.Contains(2) {
		t.Fatal("Contains(2) should be false")
	}
	if s.Len() != 1 {
		t.Fatalf("Len = %d, want 1", s.Len())
	}

	if !s.Remove(1) {
		t.Fatal("Remove existing should return true")
	}
	if s.Remove(1) {
		t.Fatal("Remove non-existing should return false")
	}
	if s.Len() != 0 {
		t.Fatalf("after remove: Len = %d, want 0", s.Len())
	}
}

func TestSet_NewSetDedup(t *testing.T) {
	s := NewSet(1, 2, 3, 2, 1)
	if s.Len() != 3 {
		t.Fatalf("NewSet with dupes: Len = %d, want 3", s.Len())
	}
	for _, v := range []int{1, 2, 3} {
		if !s.Contains(v) {
			t.Fatalf("should contain %d", v)
		}
	}
}

func TestSet_Union(t *testing.T) {
	s1 := NewSet(1, 2, 3)
	s2 := NewSet(3, 4, 5)
	u := s1.Union(s2)
	if u.Len() != 5 {
		t.Fatalf("Union Len = %d, want 5", u.Len())
	}
	for _, v := range []int{1, 2, 3, 4, 5} {
		if !u.Contains(v) {
			t.Fatalf("Union should contain %d", v)
		}
	}
}

func TestSet_Intersect(t *testing.T) {
	s1 := NewSet(1, 2, 3, 4)
	s2 := NewSet(3, 4, 5, 6)
	i := s1.Intersect(s2)
	if i.Len() != 2 {
		t.Fatalf("Intersect Len = %d, want 2", i.Len())
	}
	for _, v := range []int{3, 4} {
		if !i.Contains(v) {
			t.Fatalf("Intersect should contain %d", v)
		}
	}
}

func TestSet_Difference(t *testing.T) {
	s1 := NewSet(1, 2, 3)
	s2 := NewSet(2, 3, 4)
	d := s1.Difference(s2)
	if d.Len() != 1 {
		t.Fatalf("Difference Len = %d, want 1", d.Len())
	}
	if !d.Contains(1) {
		t.Fatal("Difference should contain 1")
	}
}

func TestSet_SymmetricDifference(t *testing.T) {
	s1 := NewSet(1, 2, 3)
	s2 := NewSet(2, 3, 4)
	sd := s1.SymmetricDifference(s2)
	if sd.Len() != 2 {
		t.Fatalf("SymmetricDifference Len = %d, want 2", sd.Len())
	}
	for _, v := range []int{1, 4} {
		if !sd.Contains(v) {
			t.Fatalf("SymmetricDifference should contain %d", v)
		}
	}
}

func TestSet_EmptySetOperations(t *testing.T) {
	empty := NewSet[int]()
	a := NewSet(1, 2)

	// empty ∪ A = A
	u := empty.Union(a)
	if !u.Equal(a) {
		t.Fatal("empty ∪ A should equal A")
	}

	// empty ∩ A = empty
	i := empty.Intersect(a)
	if i.Len() != 0 {
		t.Fatal("empty ∩ A should be empty")
	}

	// empty - A = empty
	d := empty.Difference(a)
	if d.Len() != 0 {
		t.Fatal("empty - A should be empty")
	}

	// A - empty = A
	d2 := a.Difference(empty)
	if !d2.Equal(a) {
		t.Fatal("A - empty should equal A")
	}
}

func TestSet_Relations(t *testing.T) {
	s1 := NewSet(1, 2)
	s2 := NewSet(1, 2, 3)

	if !s1.IsSubset(s2) {
		t.Fatal("{1,2} should be subset of {1,2,3}")
	}
	if s2.IsSubset(s1) {
		t.Fatal("{1,2,3} should not be subset of {1,2}")
	}
	if !s2.IsSuperset(s1) {
		t.Fatal("{1,2,3} should be superset of {1,2}")
	}
	if !s1.Equal(NewSet(1, 2)) {
		t.Fatal("{1,2} should equal {1,2}")
	}
	if s1.Equal(s2) {
		t.Fatal("{1,2} should not equal {1,2,3}")
	}
	// set is subset of itself
	if !s1.IsSubset(s1) {
		t.Fatal("set should be subset of itself")
	}
}

func TestSet_AllIteratesAllElements(t *testing.T) {
	s := NewSet(10, 20, 30)
	collected := map[int]bool{}
	for v := range s.All() {
		collected[v] = true
	}
	if len(collected) != 3 {
		t.Fatalf("All yielded %d elements, want 3", len(collected))
	}
	for _, v := range []int{10, 20, 30} {
		if !collected[v] {
			t.Fatalf("missing %d", v)
		}
	}
}

func TestSet_ToSliceUnique(t *testing.T) {
	s := NewSet("a", "b", "c")
	slice := s.ToSlice()
	if len(slice) != 3 {
		t.Fatalf("ToSlice len = %d, want 3", len(slice))
	}
	// check uniqueness by converting back to set
	s2 := NewSet(slice...)
	if s2.Len() != 3 {
		t.Fatal("ToSlice elements should be unique")
	}
}

func TestSet_CloneIndependent(t *testing.T) {
	s := NewSet(1, 2, 3)
	c := s.Clone()
	c.Add(4)
	c.Remove(1)
	if s.Contains(4) {
		t.Fatal("clone should be independent: original should not contain 4")
	}
	if !s.Contains(1) {
		t.Fatal("clone should be independent: original should still contain 1")
	}
}

func TestSet_Clear(t *testing.T) {
	s := NewSet(1, 2, 3)
	s.Clear()
	if s.Len() != 0 {
		t.Fatalf("after Clear: Len = %d, want 0", s.Len())
	}
	// can still use after clear
	s.Add(10)
	if !s.Contains(10) {
		t.Fatal("should work after Clear")
	}
}

func TestSet_SymmetricDifferenceBothEmpty(t *testing.T) {
	e1 := NewSet[int]()
	e2 := NewSet[int]()
	sd := e1.SymmetricDifference(e2)
	if sd.Len() != 0 {
		t.Fatalf("empty ⊖ empty Len = %d, want 0", sd.Len())
	}
}

func TestSet_UnionDisjoint(t *testing.T) {
	s1 := NewSet(1, 2)
	s2 := NewSet(3, 4)
	u := s1.Union(s2)
	if u.Len() != 4 {
		t.Fatalf("disjoint union Len = %d, want 4", u.Len())
	}
}

func TestSet_IntersectIdentical(t *testing.T) {
	s := NewSet(1, 2, 3)
	i := s.Intersect(s)
	if !i.Equal(s) {
		t.Fatal("identical intersect should equal itself")
	}
}

func TestSet_IntersectDisjoint(t *testing.T) {
	s1 := NewSet(1, 2)
	s2 := NewSet(3, 4)
	i := s1.Intersect(s2)
	if i.Len() != 0 {
		t.Fatalf("disjoint intersect Len = %d, want 0", i.Len())
	}
}

func TestSet_AllBreak(t *testing.T) {
	s := NewSet(10, 20, 30, 40, 50)
	var count int
	for range s.All() {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Fatalf("All break count = %d, want 3", count)
	}
}

func TestSet_StructTypeInstantiation(t *testing.T) {
	type point struct{ X, Y int }
	s := NewSetWithCapacity[point](4)
	s.Add(point{1, 2})
	s.Add(point{3, 4})
	if !s.Contains(point{1, 2}) {
		t.Fatal("should contain point{1,2}")
	}
	if s.Contains(point{1, 3}) {
		t.Fatal("should not contain point{1,3}")
	}
}

func TestSet_AlgebraInvariants(t *testing.T) {
	// inclusion-exclusion: |A ∪ B| = |A| + |B| - |A ∩ B|
	a := NewSet(1, 2, 3, 4)
	b := NewSet(3, 4, 5, 6)
	union := a.Union(b)
	inter := a.Intersect(b)
	if union.Len() != a.Len()+b.Len()-inter.Len() {
		t.Fatalf("inclusion-exclusion: %d != %d + %d - %d",
			union.Len(), a.Len(), b.Len(), inter.Len())
	}

	// A ⊖ B = (A - B) ∪ (B - A)
	sd := a.SymmetricDifference(b)
	manual := a.Difference(b).Union(b.Difference(a))
	if !sd.Equal(manual) {
		t.Fatal("SymmetricDifference != (A-B) ∪ (B-A)")
	}

	// A - B and B are disjoint
	diffAB := a.Difference(b)
	if diffAB.Intersect(b).Len() != 0 {
		t.Fatal("(A-B) ∩ B should be empty")
	}
}

func TestSet_ConcurrentRead(_ *testing.T) {
	s := NewSet[int]()
	for i := range 100 {
		s.Add(i)
	}
	done := make(chan struct{})
	for range 4 {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := range 100 {
				s.Contains(i)
			}
			for range s.All() {
				break
			}
		}()
	}
	for range 4 {
		<-done
	}
}

func TestSet_NegativeCapacityPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for negative capacity")
		}
	}()
	NewSetWithCapacity[int](-1)
}
