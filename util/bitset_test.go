package util

import (
	"testing"
)

func TestBitSet(t *testing.T) {
	b := NewBitSet(100)
	if b.Size != 100 {
		t.Errorf("Expected size 100, got %d", b.Size)
	}

	// 1. Set and Has
	b.Set(10)
	b.Set(65)
	if !b.Has(10) {
		t.Error("Expected bit 10 to be set")
	}
	if !b.Has(65) {
		t.Error("Expected bit 65 to be set")
	}
	if b.Has(11) {
		t.Error("Did not expect bit 11 to be set")
	}

	// 2. Clear
	b.Clear(10)
	if b.Has(10) {
		t.Error("Expected bit 10 to be cleared")
	}

	// 3. Count
	if b.Count() != 1 { // only 65 is set
		t.Errorf("Expected count 1, got %d", b.Count())
	}

	// 4. Union
	other := NewBitSet(100)
	other.Set(10)
	other.Set(20)
	b.Union(other)
	if !b.Has(10) || !b.Has(20) || !b.Has(65) {
		t.Error("Union failed")
	}

	// 5. Intersect
	third := NewBitSet(100)
	third.Set(20)
	third.Set(30)
	b.Intersect(third)
	if b.Has(10) || b.Has(65) || !b.Has(20) {
		t.Error("Intersection failed")
	}

	// 6. Equal
	b2 := NewBitSet(100)
	b2.Set(20)
	if !b.Equal(b2) {
		t.Error("Equality check failed")
	}

	// 7. Reset
	b.Reset()
	if b.Count() != 0 {
		t.Error("Reset failed")
	}
}
