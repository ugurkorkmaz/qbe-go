package opt

import (
	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

// PhiElim removes Phi nodes by inserting copies in predecessor blocks.
// This is also known as "Out-of-SSA" transformation.
func PhiElim(f *ir.Function) {
	// To avoid edge splitting for now, we'll assume the CFG is prepared
	// or we'll just insert in predecessors (risky for multiple successors).
	// Proper way: split critical edges.

	for _, b := range f.Blocks {
		if len(b.Phis) == 0 {
			continue
		}

		for _, p := range b.Phis {
			for i, arg := range p.Args {
				pred := p.Blks[i]

				// Insert copy in predecessor
				copyIns := ir.Instruction{
					Op:  ir.Ocopy,
					Cls: p.Cls,
					To:  p.To,
					Arg: [2]ir.Ref{arg, ir.Undef},
				}

				// Rule: if pred has multiple successors, we should split the edge.
				// For this demo/first pass, we'll just append to the block's instructions.
				pred.Ins = append(pred.Ins, copyIns)
			}
		}
		// Remove Phis from the block
		b.Phis = nil
	}

	// Since we changed instructions, block IDs and Liveness might need update.
	analysis.FillRPO(f)
	analysis.Liveness(f)
}
