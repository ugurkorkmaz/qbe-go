package opt

import (
	"fmt"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

// PhiElim removes Phi nodes by inserting copies in predecessor blocks.
// Critical edges (predecessor has multiple successors) are split by inserting
// an intermediate block so that the copy only executes on the correct path.
func PhiElim(f *ir.Function) {
	splitCache := make(map[[2]uint32]*ir.Block)

	for _, b := range f.Blocks {
		if len(b.Phis) == 0 {
			continue
		}

		for _, p := range b.Phis {
			for i, arg := range p.Args {
				pred := p.Blks[i]

				copyIns := ir.Instruction{
					Op:  ir.Ocopy,
					Cls: p.Cls,
					To:  p.To,
					Arg: [2]ir.Ref{arg, ir.Undef},
				}

				if pred.S2 == nil {
					// Safe: predecessor has a single successor, append copy directly.
					pred.Ins = append(pred.Ins, copyIns)
				} else {
					// Critical edge: insert a split block so the copy only runs on this path.
					key := [2]uint32{pred.Id, b.Id}
					split, exists := splitCache[key]
					if !exists {
						split = &ir.Block{
							Name: fmt.Sprintf("crit_%d_%d", pred.Id, b.Id),
							Id:   f.NBlk,
						}
						f.NBlk++
						split.Jmp = ir.Jump{Type: ir.Jjmp}
						split.S1 = b

						if pred.S1 == b {
							pred.S1 = split
						} else {
							pred.S2 = split
						}

						splitCache[key] = split
						f.Blocks = append(f.Blocks, split)
					}
					split.Ins = append(split.Ins, copyIns)
				}
			}
		}
		b.Phis = nil
	}

	analysis.FillPreds(f)
	analysis.FillRPO(f)
	analysis.Liveness(f)
}
