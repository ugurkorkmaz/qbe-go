package opt

import (
	"fmt"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

// PhiElim removes Phi nodes by inserting copies in predecessor blocks.
// It handles parallel copy semantics by using intermediate temporaries.
func PhiElim(f *ir.Function) {
	type phiCopy struct {
		to  ir.Ref
		arg ir.Ref
		cls ir.Class
	}
	
	// Map: (Predecessor Block) -> (Successor Block) -> []phiCopy
	edgeCopies := make(map[*ir.Block]map[*ir.Block][]phiCopy)

	for _, b := range f.Blocks {
		if len(b.Phis) == 0 {
			continue
		}

		for _, p := range b.Phis {
			for i, arg := range p.Args {
				pred := p.Blks[i]
				if edgeCopies[pred] == nil {
					edgeCopies[pred] = make(map[*ir.Block][]phiCopy)
				}
				edgeCopies[pred][b] = append(edgeCopies[pred][b], phiCopy{p.To, arg, p.Cls})
			}
		}
		b.Phis = nil // Remove Phi nodes from block
	}

	splitCache := make(map[[2]uint32]*ir.Block)

	for pred, succs := range edgeCopies {
		for succ, copies := range succs {
			// To implement parallel copy semantics safely:
			// 1. Copy all Phi arguments to fresh temporaries.
			// 2. Copy those temporaries to the final Phi 'to' destinations.
			var copyIns []ir.Instruction
			for _, cp := range copies {
				tmp := f.NewTmp("phi", cp.cls)
				// tmp = copy arg
				copyIns = append(copyIns, ir.Instruction{
					Op:  ir.Ocopy,
					Cls: cp.cls,
					To:  tmp,
					Arg: [2]ir.Ref{cp.arg, ir.Undef},
				})
				// to = copy tmp
				copyIns = append(copyIns, ir.Instruction{
					Op:  ir.Ocopy,
					Cls: cp.cls,
					To:  cp.to,
					Arg: [2]ir.Ref{tmp, ir.Undef},
				})
			}

			// Decide where to insert these copy instructions
			if pred.S2 == nil {
				// Single successor: Insert BEFORE the jump
				pred.Ins = append(pred.Ins, copyIns...)
			} else {
				// Multiple successors: Edge splitting (already creates a block with its own jump)
				key := [2]uint32{pred.Id, succ.Id}
				split, exists := splitCache[key]
				if !exists {
					split = &ir.Block{
						Name: fmt.Sprintf("crit_%d_%d", pred.Id, succ.Id),
						Id:   f.NBlk,
					}
					f.NBlk++
					split.Jmp = ir.Jump{Type: ir.Jjmp, Arg: ir.Undef}
					split.S1 = succ

					if pred.S1 == succ {
						pred.S1 = split
					} else if pred.S2 == succ {
						pred.S2 = split
					}
					
					f.Blocks = append(f.Blocks, split)
					splitCache[key] = split
				}
				// Append to crit block (it's empty except for jump)
				split.Ins = append(split.Ins, copyIns...)
			}
		}
	}

	// Re-run analyses since CFG and instructions have changed
	analysis.FillPreds(f)
	analysis.FillRPO(f)
	analysis.Liveness(f)
}
