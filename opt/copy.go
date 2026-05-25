package opt

import (
	"github.com/ugurkorkmaz/qbe-go/ir"
)

// CopyProp performs Global Copy Propagation on SSA IR.
// It removes redundant copies of the form '%x = copy %y'
// by replacing all occurrences of %x with %y.
func CopyProp(f *ir.Function) {
	// 1. Build a 'copy-of' map
	cpy := make([]ir.Ref, f.NTmp)
	for i := range cpy {
		cpy[i] = ir.Ref{Kind: ir.RTmp, Val: uint32(i)}
	}

	changed := true
	for changed {
		changed = false
		for _, b := range f.Blocks {
			// Handle instructions
			for i := range b.Ins {
				ins := &b.Ins[i]
				if ins.Op == ir.Ocopy && ins.To.IsTmp() && ins.Arg[0].IsTmp() {
					oldRef := ins.To.Val
					newRef := findRoot(ins.Arg[0], cpy)
					if cpy[oldRef] != newRef {
						cpy[oldRef] = newRef
						changed = true
					}
				}
			}
			// Handle Phis: if all args of a Phi are the same, it's a copy
			for _, p := range b.Phis {
				if len(p.Args) == 0 {
					continue
				}
				root := findRoot(p.Args[0], cpy)
				redundant := true
				for _, arg := range p.Args[1:] {
					if findRoot(arg, cpy) != root {
						redundant = false
						break
					}
				}
				if redundant {
					oldRef := p.To.Val
					if cpy[oldRef] != root {
						cpy[oldRef] = root
						changed = true
					}
				}
			}
		}
	}

	// 2. Apply replacements
	for _, b := range f.Blocks {
		// Instructions
		var newIns []ir.Instruction
		for i := range b.Ins {
			ins := &b.Ins[i]
			// Replace uses in any instruction
			for n := 0; n < 3; n++ {
				if ins.Arg[n].IsTmp() {
					ins.Arg[n] = findRoot(ins.Arg[n], cpy)
				}
			}
			// If this was a redundant copy, turn it into NOP
			if ins.Op == ir.Ocopy && ins.To.IsTmp() {
				if findRoot(ins.To, cpy) != ins.To {
					ins.Op = ir.Onop
					continue
				}
			}
			newIns = append(newIns, *ins)
		}
		b.Ins = newIns

		// Phis
		var newPhis []*ir.Phi
		for _, p := range b.Phis {
			if findRoot(p.To, cpy) != p.To {
				continue // redundant Phi
			}
			for i := range p.Args {
				if p.Args[i].IsTmp() {
					p.Args[i] = findRoot(p.Args[i], cpy)
				}
			}
			newPhis = append(newPhis, p)
		}
		b.Phis = newPhis

		// Jump
		if b.Jmp.Arg.IsTmp() {
			b.Jmp.Arg = findRoot(b.Jmp.Arg, cpy)
		}
	}
}

func findRoot(r ir.Ref, cpy []ir.Ref) ir.Ref {
	if !r.IsTmp() {
		return r
	}
	root := r
	for cpy[root.Val] != root {
		root = cpy[root.Val]
	}
	return root
}
