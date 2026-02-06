package opt

import (
	"github.com/ugurkorkmaz/qbe-go/ir"
	"github.com/ugurkorkmaz/qbe-go/util"
)

// DCE performs Iterative Dead Code Elimination.
// It repeatedly scans blocks backwards to remove dead instructions until no more changes occur.
func DCE(f *ir.Function) {
	changed := true
	for changed {
		changed = false
		for _, b := range f.Blocks {
			// Start with LiveOut set (from Liveness analysis)
			// Note: Ideally, we should re-run Liveness analysis if global liveness changed.
			// However, simple local checking with convergence is often sufficient for local DCE.
			live := util.NewBitSet(f.NTmp)
			live.Copy(b.Out)

			// Jumps use value
			if b.Jmp.Arg.Kind == ir.RTmp {
				live.Set(b.Jmp.Arg.Val)
			}

			// Scan Instructions Backwards
			isDead := make([]bool, len(b.Ins))

			for i := len(b.Ins) - 1; i >= 0; i-- {
				ins := &b.Ins[i]
				dead := false

				// Check if instruction defines a Temp that is NOT live
				// And has no side effects (like Store, Call)
				if ins.To.Kind == ir.RTmp {
					if !live.Has(ins.To.Val) && !ins.HasSideEffects() {
						dead = true
						changed = true
					}
				} else if ins.To.IsUndef() && !ins.HasSideEffects() {
					// Instruction produces nothing and has no side effects? Dead.
					dead = true
					changed = true
				}

				if !dead {
					// Instruction is live.
					// It kills liveness of its Output (Def)
					if ins.To.Kind == ir.RTmp {
						live.Clear(ins.To.Val)
					}
					// It generates liveness for its Inputs (Uses)
					for _, arg := range ins.Arg {
						if arg.Kind == ir.RTmp {
							live.Set(arg.Val)
						}
					}
				}
				isDead[i] = dead
			}

			// Rebuild instructions if changed
			if changed {
				newIns := make([]ir.Instruction, 0, len(b.Ins))
				for i, ins := range b.Ins {
					if !isDead[i] {
						newIns = append(newIns, ins)
					}
				}
				b.Ins = newIns
			}

			// Handle Phis
			// Phis define variables used inside the block.
			// But Phis depend on b.In liveness.
			// Iterative backward pass updates 'live' set up to the top of the block.
			// 'live' now represents the LiveIn set effectively.

			newPhis := make([]*ir.Phi, 0, len(b.Phis))
			for _, p := range b.Phis {
				if p.To.Kind == ir.RTmp && live.Has(p.To.Val) {
					newPhis = append(newPhis, p)
					// Phi inputs are uses in Predecessor blocks.
					// We don't update pred blocks in this pass, but next iteration (or previous) handles it.
				} else if p.To.Kind != ir.RTmp {
					newPhis = append(newPhis, p)
				} else {
					// Phi is dead
					changed = true
				}
			}
			b.Phis = newPhis
		}
	}
}
