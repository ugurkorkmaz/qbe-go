package opt

import (
	"fmt"

	"github.com/ugurkorkmaz/qbe-go/ir"
)

// GVN performs Global Value Numbering on SSA IR.
// It detects redundant computations and replaces them with
// previously computed values.
func GVN(f *ir.Function) {
	// Map from value expression to its canonical temporary
	valueTable := make(map[string]ir.Ref)
	// Map from temporary to its canonical replacement
	replacements := make([]ir.Ref, f.NTmp)
	for i := range replacements {
		replacements[i] = ir.Ref{Kind: ir.RTmp, Val: uint32(i)}
	}

	// We process blocks in Dominator Tree order (DFS)
	var walk func(b *ir.Block)
	walk = func(b *ir.Block) {
		// Track entries added in this scope to revert later
		var scopeKeys []string

		addValue := func(expr string, val ir.Ref) {
			if _, exists := valueTable[expr]; !exists {
				valueTable[expr] = val
				scopeKeys = append(scopeKeys, expr)
			}
		}

		// 1. Process Phis
		for _, p := range b.Phis {
			expr := formatPhiExpr(p, replacements)
			if canonical, ok := valueTable[expr]; ok {
				replacements[p.To.Val] = canonical
			} else {
				addValue(expr, p.To)
			}
		}

		// 2. Process Instructions
		var newIns []ir.Instruction
		for i := range b.Ins {
			ins := &b.Ins[i]
			// Replace arguments first
			for n := 0; n < 3; n++ {
				if ins.Arg[n].IsTmp() {
					ins.Arg[n] = replacements[ins.Arg[n].Val]
				}
			}

			if ins.Op == ir.Onop {
				continue
			}

			// Check if this expression was already computed
			if !ins.To.IsUndef() && !ins.HasSideEffects() {
				expr := formatInsExpr(ins)
				if canonical, ok := valueTable[expr]; ok {
					replacements[ins.To.Val] = canonical
					continue // redundant instruction
				} else {
					addValue(expr, ins.To)
				}
			}
			newIns = append(newIns, *ins)
		}
		b.Ins = newIns

		// 3. Recurse down Dom Tree
		for s := b.Dom; s != nil; s = s.DLink {
			walk(s)
		}

		// 4. Leave Scope: Remove entries added in this block
		for _, key := range scopeKeys {
			delete(valueTable, key)
		}
	}

	walk(f.Start)

	// Finally, update Jump arguments and Phis
	for _, b := range f.Blocks {
		if b.Jmp.Arg.IsTmp() {
			b.Jmp.Arg = replacements[b.Jmp.Arg.Val]
		}
		for _, p := range b.Phis {
			for i := range p.Args {
				if p.Args[i].IsTmp() {
					p.Args[i] = replacements[p.Args[i].Val]
				}
			}
		}
	}
}

func formatInsExpr(ins *ir.Instruction) string {
	// Commutative ops should normalize argument order
	a1, a2, a3 := ins.Arg[0], ins.Arg[1], ins.Arg[2]
	if isCommutative(ins.Op) && a1.Val > a2.Val {
		a1, a2 = a2, a1
	}
	return fmt.Sprintf("%d:%d:%v:%v:%v", ins.Op, ins.Cls, a1, a2, a3)
}

func formatPhiExpr(p *ir.Phi, replacements []ir.Ref) string {
	// Normalizing Phis is harder, but a simple version is to just
	// use 'phi' + sorted args if we don't care about block order (we do, usually)
	// For now, Phis are only redundant if they are identical including blocks.
	return fmt.Sprintf("phi:%d:%v:%v", p.Cls, p.Args, p.Blks)
}

func isCommutative(op ir.Opcode) bool {
	switch op {
	case ir.Oadd, ir.Omul, ir.Oand, ir.Oor, ir.Oxor, ir.Oceqw, ir.Oceql:
		return true
	}
	return false
}
