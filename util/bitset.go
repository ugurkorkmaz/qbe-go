package util

import "math/bits"

// BitSet is a high-performance bit set used for dataflow analysis.
type BitSet struct {
	Words []uint64
	Size  uint32 // Number of bits
}

// NewBitSet creates a new BitSet that can hold 'size' bits.
func NewBitSet(size uint32) BitSet {
	nwords := (size + 63) / 64
	return BitSet{
		Words: make([]uint64, nwords),
		Size:  size,
	}
}

// Set sets the i-th bit to 1.
func (b *BitSet) Set(i uint32) {
	if i >= b.Size {
		return
	}
	b.Words[i/64] |= 1 << (i % 64)
}

// Clear sets the i-th bit to 0.
func (b *BitSet) Clear(i uint32) {
	if i >= b.Size {
		return
	}
	b.Words[i/64] &^= 1 << (i % 64)
}

// Has returns true if the i-th bit is 1.
func (b *BitSet) Has(i uint32) bool {
	if i >= b.Size {
		return false
	}
	return (b.Words[i/64] & (1 << (i % 64))) != 0
}

// Reset clears all bits.
func (b *BitSet) Reset() {
	for i := range b.Words {
		b.Words[i] = 0
	}
}

// Union performs bitwise OR: b |= other.
func (b *BitSet) Union(other BitSet) bool {
	changed := false
	for i := range b.Words {
		old := b.Words[i]
		b.Words[i] |= other.Words[i]
		if b.Words[i] != old {
			changed = true
		}
	}
	return changed
}

// Intersect performs bitwise AND: b &= other.
func (b *BitSet) Intersect(other BitSet) bool {
	changed := false
	for i := range b.Words {
		old := b.Words[i]
		b.Words[i] &= other.Words[i]
		if b.Words[i] != old {
			changed = true
		}
	}
	return changed
}

// Diff performs bitwise subtraction: b &= ^other.
func (b *BitSet) Diff(other BitSet) bool {
	changed := false
	for i := range b.Words {
		old := b.Words[i]
		b.Words[i] &^= other.Words[i]
		if b.Words[i] != old {
			changed = true
		}
	}
	return changed
}

// Copy copies bits from other.
func (b *BitSet) Copy(other BitSet) {
	copy(b.Words, other.Words)
}

// Count returns the number of set bits.
func (b *BitSet) Count() (n uint32) {
	for _, w := range b.Words {
		n += uint32(bits.OnesCount64(w))
	}
	return n
}

// Equal returns true if both sets have the same bits set.
func (b *BitSet) Equal(other BitSet) bool {
	if len(b.Words) != len(other.Words) {
		return false
	}
	for i := range b.Words {
		if b.Words[i] != other.Words[i] {
			return false
		}
	}
	return true
}
