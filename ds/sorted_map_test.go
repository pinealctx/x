package ds_test

import (
	"slices"
	"testing"

	"github.com/pinealctx/x/ds"
)

// smItem is a test value where ID is the map key and Name is the sort key.
type smItem struct {
	ID   int
	Name string
}

func newSortedMap() *ds.SortedMap[int, smItem] {
	return ds.NewSortedMap(
		func(v smItem) int { return v.ID },
		func(a, b smItem) bool { return a.Name < b.Name },
	)
}

func TestSortedMap_EmptyOps(t *testing.T) {
	m := newSortedMap()
	if m.Len() != 0 {
		t.Fatal("expected Len 0")
	}
	if _, ok := m.Get(1); ok {
		t.Fatal("Get on empty should return false")
	}
	if m.Has(1) {
		t.Fatal("Has on empty should return false")
	}
	if m.Delete(1) {
		t.Fatal("Delete on empty should return false")
	}
	var ascend []smItem
	for v := range m.Ascend() {
		ascend = append(ascend, v)
	}
	if len(ascend) != 0 {
		t.Fatal("Ascend on empty should yield nothing")
	}
	// verify all range iterators terminate cleanly on empty map
	for range m.AscendFrom(smItem{Name: "a"}) { //nolint:revive // intentional empty range: verifying empty iterator
	}
	for range m.AscendAfter(smItem{Name: "a"}) { //nolint:revive // intentional empty range: verifying empty iterator
	}
	for range m.Descend() { //nolint:revive // intentional empty range: verifying empty iterator
	}
	for range m.DescendFrom(smItem{Name: "z"}) { //nolint:revive // intentional empty range: verifying empty iterator
	}
	for range m.DescendBefore(smItem{Name: "z"}) { //nolint:revive // intentional empty range: verifying empty iterator
	}
}

func TestSortedMap_SetReturnValue(t *testing.T) {
	m := newSortedMap()
	if !m.Set(smItem{1, "alpha"}) {
		t.Fatal("first Set should return true (new insert)")
	}
	if m.Set(smItem{1, "alpha-updated"}) {
		t.Fatal("second Set with same key should return false (update)")
	}
}

func TestSortedMap_GetHasDelete(t *testing.T) {
	m := newSortedMap()
	m.Set(smItem{1, "alpha"})
	m.Set(smItem{2, "beta"})

	v, ok := m.Get(1)
	if !ok || v.Name != "alpha" {
		t.Fatalf("Get(1) = %v, %v; want alpha, true", v, ok)
	}
	if !m.Has(2) {
		t.Fatal("Has(2) should be true")
	}
	if m.Has(99) {
		t.Fatal("Has(99) should be false")
	}
	if !m.Delete(1) {
		t.Fatal("Delete(1) should return true")
	}
	if m.Delete(1) {
		t.Fatal("Delete(1) again should return false")
	}
	if m.Len() != 1 {
		t.Fatalf("Len = %d; want 1", m.Len())
	}
}

func TestSortedMap_UpdatePreservesOrder(t *testing.T) {
	m := newSortedMap()
	m.Set(smItem{1, "charlie"})
	m.Set(smItem{2, "alpha"})
	m.Set(smItem{3, "beta"})
	// Update ID=1 to sort before alpha
	m.Set(smItem{1, "aaa"})

	// Verify Get returns the new value
	v, ok := m.Get(1)
	if !ok || v.Name != "aaa" {
		t.Fatalf("Get(1) after update = %v, %v; want aaa, true", v, ok)
	}

	// Verify Ascend order reflects the updated sort key
	var names []string
	for v := range m.Ascend() {
		names = append(names, v.Name)
	}
	want := []string{"aaa", "alpha", "beta"}
	if !slices.Equal(names, want) {
		t.Fatalf("Ascend after update = %v; want %v", names, want)
	}
}

func TestSortedMap_AscendDescend(t *testing.T) {
	m := newSortedMap()
	m.Set(smItem{1, "charlie"})
	m.Set(smItem{2, "alpha"})
	m.Set(smItem{3, "beta"})

	var asc []string
	for v := range m.Ascend() {
		asc = append(asc, v.Name)
	}
	if !slices.Equal(asc, []string{"alpha", "beta", "charlie"}) {
		t.Fatalf("Ascend = %v", asc)
	}

	var desc []string
	for v := range m.Descend() {
		desc = append(desc, v.Name)
	}
	if !slices.Equal(desc, []string{"charlie", "beta", "alpha"}) {
		t.Fatalf("Descend = %v", desc)
	}
}

func TestSortedMap_AscendFrom(t *testing.T) {
	m := newSortedMap()
	m.Set(smItem{1, "alpha"})
	m.Set(smItem{2, "charlie"})
	m.Set(smItem{3, "echo"})

	tests := []struct {
		pivot string
		want  []string
	}{
		{"charlie", []string{"charlie", "echo"}},        // pivot exists
		{"bravo", []string{"charlie", "echo"}},          // pivot between elements
		{"aaa", []string{"alpha", "charlie", "echo"}},   // pivot smaller than all
		{"alpha", []string{"alpha", "charlie", "echo"}}, // pivot == first element
		{"zzz", nil}, // pivot larger than all
	}
	for _, tt := range tests {
		t.Run("pivot="+tt.pivot, func(t *testing.T) {
			var got []string
			for v := range m.AscendFrom(smItem{Name: tt.pivot}) {
				got = append(got, v.Name)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("AscendFrom(%q) = %v; want %v", tt.pivot, got, tt.want)
			}
		})
	}
}

func TestSortedMap_AscendAfter(t *testing.T) {
	m := newSortedMap()
	m.Set(smItem{1, "alpha"})
	m.Set(smItem{2, "charlie"})
	m.Set(smItem{3, "echo"})

	tests := []struct {
		pivot string
		want  []string
	}{
		{"charlie", []string{"echo"}},                 // pivot exists — exclude it
		{"bravo", []string{"charlie", "echo"}},        // pivot between elements
		{"aaa", []string{"alpha", "charlie", "echo"}}, // pivot smaller than all
		{"alpha", []string{"charlie", "echo"}},        // pivot == first element
		{"echo", nil},                                 // pivot == last element
		{"zzz", nil},                                  // pivot larger than all
	}
	for _, tt := range tests {
		t.Run("pivot="+tt.pivot, func(t *testing.T) {
			var got []string
			for v := range m.AscendAfter(smItem{Name: tt.pivot}) {
				got = append(got, v.Name)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("AscendAfter(%q) = %v; want %v", tt.pivot, got, tt.want)
			}
		})
	}
}

func TestSortedMap_DescendFrom(t *testing.T) {
	m := newSortedMap()
	m.Set(smItem{1, "alpha"})
	m.Set(smItem{2, "charlie"})
	m.Set(smItem{3, "echo"})

	tests := []struct {
		pivot string
		want  []string
	}{
		{"charlie", []string{"charlie", "alpha"}},      // pivot exists
		{"bravo", []string{"alpha"}},                   // pivot between elements
		{"zzz", []string{"echo", "charlie", "alpha"}},  // pivot larger than all
		{"echo", []string{"echo", "charlie", "alpha"}}, // pivot == last element
		{"aaa", nil}, // pivot smaller than all
	}
	for _, tt := range tests {
		t.Run("pivot="+tt.pivot, func(t *testing.T) {
			var got []string
			for v := range m.DescendFrom(smItem{Name: tt.pivot}) {
				got = append(got, v.Name)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("DescendFrom(%q) = %v; want %v", tt.pivot, got, tt.want)
			}
		})
	}
}

func TestSortedMap_DescendBefore(t *testing.T) {
	m := newSortedMap()
	m.Set(smItem{1, "alpha"})
	m.Set(smItem{2, "charlie"})
	m.Set(smItem{3, "echo"})

	tests := []struct {
		pivot string
		want  []string
	}{
		{"charlie", []string{"alpha"}},                // pivot exists — exclude it
		{"bravo", []string{"alpha"}},                  // pivot between elements
		{"zzz", []string{"echo", "charlie", "alpha"}}, // pivot larger than all
		{"echo", []string{"charlie", "alpha"}},        // pivot == last element
		{"alpha", nil},                                // pivot == first element
		{"aaa", nil},                                  // pivot smaller than all
	}
	for _, tt := range tests {
		t.Run("pivot="+tt.pivot, func(t *testing.T) {
			var got []string
			for v := range m.DescendBefore(smItem{Name: tt.pivot}) {
				got = append(got, v.Name)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("DescendBefore(%q) = %v; want %v", tt.pivot, got, tt.want)
			}
		})
	}
}

func TestSortedMap_EarlyBreak(t *testing.T) {
	m := newSortedMap()
	for i := range 10 {
		m.Set(smItem{i, string(rune('a' + i))})
	}

	count := 0
	for range m.Ascend() {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Fatalf("early break: got %d iterations, want 3", count)
	}

	count = 0
	for range m.Descend() {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Fatalf("early break descend: got %d iterations, want 3", count)
	}
}

func TestSortedMap_ClearAndReuse(t *testing.T) {
	m := newSortedMap()
	m.Set(smItem{1, "alpha"})
	m.Set(smItem{2, "beta"})
	m.Clear()
	if m.Len() != 0 {
		t.Fatal("Len after Clear should be 0")
	}
	if m.Has(1) {
		t.Fatal("Has after Clear should be false")
	}
	var items []smItem
	for v := range m.Ascend() {
		items = append(items, v)
	}
	if len(items) != 0 {
		t.Fatal("Ascend after Clear should yield nothing")
	}

	// Reuse
	m.Set(smItem{3, "gamma"})
	if m.Len() != 1 {
		t.Fatal("Len after reuse should be 1")
	}
}

// TestSortedMap_DecoupledKeys verifies that the map lookup key (int ID)
// and the B-tree sort key (string Path) can be completely different fields.
func TestSortedMap_DecoupledKeys(t *testing.T) {
	type file struct {
		ID   int
		Path string
	}
	m := ds.NewSortedMap(
		func(v file) int { return v.ID },
		func(a, b file) bool { return a.Path < b.Path },
	)
	m.Set(file{10, "/z/file"})
	m.Set(file{20, "/a/file"})
	m.Set(file{30, "/m/file"})

	// Random access by ID
	v, ok := m.Get(20)
	if !ok || v.Path != "/a/file" {
		t.Fatalf("Get by ID: got %v, %v", v, ok)
	}

	// Ordered iteration by Path
	var paths []string
	for f := range m.Ascend() {
		paths = append(paths, f.Path)
	}
	if !slices.Equal(paths, []string{"/a/file", "/m/file", "/z/file"}) {
		t.Fatalf("Ascend by Path = %v", paths)
	}

	// Delete by ID, verify B-tree order is maintained
	m.Delete(20)
	paths = paths[:0]
	for f := range m.Ascend() {
		paths = append(paths, f.Path)
	}
	if !slices.Equal(paths, []string{"/m/file", "/z/file"}) {
		t.Fatalf("Ascend after Delete = %v", paths)
	}
}
