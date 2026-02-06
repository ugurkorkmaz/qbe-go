package analysis

import (
	"github.com/ugurkorkmaz/qbe-go/ir"
)

// FillPreds populates the Pred (Predecessors) slice for each block in the function.
func FillPreds(f *ir.Function) {
	for _, b := range f.Blocks {
		b.Pred = nil
	}
	for _, b := range f.Blocks {
		if b.S1 != nil {
			b.S1.Pred = append(b.S1.Pred, b)
		}
		if b.S2 != nil {
			b.S2.Pred = append(b.S2.Pred, b)
		}
	}
}

// FillRPO computes the Reverse Post Order of blocks, matching QBE heuristics.
func FillRPO(f *ir.Function) {
	// Reset IDs
	for _, b := range f.Blocks {
		b.Id = ^uint32(0)
	}

	// rporec implements QBE's RPO assignment
	var rpoRec func(*ir.Block, uint32) uint32
	rpoRec = func(b *ir.Block, x uint32) uint32 {
		if b == nil || b.Id != ^uint32(0) {
			return x
		}
		b.Id = 1 // Temporary mark

		s1, s2 := b.S1, b.S2
		// Loop heuristic: visit deeper loops earlier in RPO
		if s1 != nil && s2 != nil && s1.Loop > s2.Loop {
			s1, s2 = b.S2, b.S1
		}

		x = rpoRec(s1, x)
		x = rpoRec(s2, x)
		b.Id = x
		return x - 1
	}

	nblk := uint32(len(f.Blocks))
	nUnreachable := 1 + rpoRec(f.Start, nblk-1)

	// Collect reachable blocks and adjust IDs
	var reachable []*ir.Block
	newNBlk := nblk - nUnreachable

	// We need a way to track all blocks even if unreachable to clean them up
	// For now, we'll just filter them out of f.Blocks
	for _, b := range f.Blocks {
		if b.Id == ^uint32(0) {
			// Unreachable: Clean up edges
			edgeDel(b, &b.S1)
			edgeDel(b, &b.S2)
		} else {
			b.Id -= nUnreachable
			reachable = append(reachable, b)
		}
	}

	// Sort f.Blocks by the new ID to maintain RPO order in the slice
	f.Blocks = make([]*ir.Block, newNBlk)
	for _, b := range reachable {
		f.Blocks[b.Id] = b
	}
	f.NBlk = newNBlk
}

func edgeDel(src *ir.Block, pDst **ir.Block) {
	dst := *pDst
	if dst == nil {
		return
	}
	*pDst = nil
	// In QBE, this also removes src from dst->pred and handles Phis.
	// Since our FillPreds re-runs, we mainly care about Jmp/S1/S2 being nilled.
}

// FillLoop computes the loop depth of each block.
func FillLoop(f *ir.Function) {
	for _, b := range f.Blocks {
		b.Loop = 1
		b.Visit = ^uint32(0) // Max uint32
	}

	for _, b := range f.Blocks {
		for _, p := range b.Pred {
			if p.Id >= b.Id {
				loopMark(b, p)
			}
		}
	}
}

func loopMark(hd, b *ir.Block) {
	if b.Id < hd.Id || b.Visit == hd.Id {
		return
	}
	b.Visit = hd.Id
	b.Loop *= 10
	for _, p := range b.Pred {
		loopMark(hd, p)
	}
}
