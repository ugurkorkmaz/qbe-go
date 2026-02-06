package analysis

import (
	"github.com/ugurkorkmaz/qbe-go/ir"
)

// FillDom computes immediate dominators (Idom) for each block in the function.
// It uses "A Simple, Fast Dominance Algorithm" by Cooper, Harvey, and Kennedy.
func FillDom(f *ir.Function) {
	if len(f.Blocks) == 0 {
		return
	}

	for _, b := range f.Blocks {
		b.Idom = nil
		b.Dom = nil
		b.DLink = nil
	}

	// Start block dominates itself by convention in this algorithm's initialization
	// but QBE sets it to 0. We'll follow QBE's lead.

	changed := true
	for changed {
		changed = false
		// Process blocks in RPO starting from the second block (f.Blocks[0] is Start)
		for i := 1; i < len(f.Blocks); i++ {
			b := f.Blocks[i]
			var newIdom *ir.Block

			// Find first processed predecessor
			for _, p := range b.Pred {
				if p.Idom != nil || p == f.Start {
					newIdom = intersect(newIdom, p)
				}
			}

			if b.Idom != newIdom {
				b.Idom = newIdom
				changed = true
			}
		}
	}

	// Build the dominator tree links (Dom and DLink)
	// Dom is the head of a linked list of blocks immediately dominated by this block.
	// DLink is the next sibling in that list.
	for _, b := range f.Blocks {
		if d := b.Idom; d != nil {
			b.DLink = d.Dom
			d.Dom = b
		}
	}
}

// intersect finds the common ancestor of b1 and b2 in the dominator tree.
func intersect(b1, b2 *ir.Block) *ir.Block {
	if b1 == nil {
		return b2
	}
	for b1 != b2 {
		// Use RPO IDs (b1.Id < b2.Id means b1 is higher in RPO)
		// Cooper's paper uses post-order IDs where larger means higher.
		// Since we use RPO, smaller ID means higher in the tree.
		for b1.Id > b2.Id {
			b1 = b1.Idom
		}
		for b2.Id > b1.Id {
			b2 = b2.Idom
		}
	}
	return b1
}

// IsSdom returns true if b1 strictly dominates b2.
func IsSdom(b1, b2 *ir.Block) bool {
	if b1 == b2 {
		return false
	}
	for b2 != nil && b2.Id > b1.Id {
		b2 = b2.Idom
	}
	return b1 == b2
}

// IsDom returns true if b1 dominates b2.
func IsDom(b1, b2 *ir.Block) bool {
	return b1 == b2 || IsSdom(b1, b2)
}

// FillFron computes the dominance frontier for each block.
func FillFron(f *ir.Function) {
	for _, b := range f.Blocks {
		b.Fron = nil
	}

	for _, b := range f.Blocks {
		// Check both successors
		if b.S1 != nil {
			computeFron(b, b.S1)
		}
		if b.S2 != nil {
			computeFron(b, b.S2)
		}
	}
}

func computeFron(b, succ *ir.Block) {
	// For each predecessor-successor edge (b, succ),
	// if b does not strictly dominate succ, succ is in b's dominance frontier.
	// We then crawl up from b in the dominator tree.
	for a := b; a != nil && !IsSdom(a, succ); a = a.Idom {
		addFron(a, succ)
	}
}

func addFron(a, b *ir.Block) {
	for _, f := range a.Fron {
		if f == b {
			return
		}
	}
	a.Fron = append(a.Fron, b)
}
