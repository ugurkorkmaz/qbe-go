package analysis

import (
	"fmt"

	"github.com/ugurkorkmaz/qbe-go/ir"
)

// VerifySSA ensures that the function IR respects SSA invariants.
func VerifySSA(f *ir.Function) error {
	// First, check for multiple definitions
	defs := make([]int, f.NTmp)
	for _, b := range f.Blocks {
		for _, p := range b.Phis {
			if p.To.IsTmp() && p.To.Val != 0 {
				defs[p.To.Val]++
			}
		}
		for _, ins := range b.Ins {
			if ins.To.IsTmp() && ins.To.Val != 0 {
				defs[ins.To.Val]++
			}
		}
	}

	for t, count := range defs {
		if t < 64 { continue } // Skip physical registers
		if count > 1 {
			return fmt.Errorf("temporary %%%d defined %d times (SSA requires exactly one definition)", t, count)
		}
	}

	// Second, check dominance invariant
	for _, b := range f.Blocks {
		for _, ins := range b.Ins {
			for _, arg := range ins.Arg {
				if arg.IsTmp() && arg.Val != 0 {
					if err := checkDom(f, arg.Val, b); err != nil {
						return fmt.Errorf("instruction %s in @%s: %v", ins.Op, b.Name, err)
					}
				}
			}
		}

		if b.Jmp.Arg.IsTmp() && b.Jmp.Arg.Val != 0 {
			if err := checkDom(f, b.Jmp.Arg.Val, b); err != nil {
				return fmt.Errorf("jump in @%s: %v", b.Name, err)
			}
		}
	}

	return nil
}

func checkDom(f *ir.Function, tid uint32, useBlock *ir.Block) error {
	defBlock := findDefBlock(f, tid)
	if defBlock == nil {
		return fmt.Errorf("temporary %%%d is used but never defined", tid)
	}

	if !IsDom(defBlock, useBlock) {
		return fmt.Errorf("use of %%%d in @%s is not dominated by its definition in @%s",
			tid, useBlock.Name, defBlock.Name)
	}
	return nil
}

func findDefBlock(f *ir.Function, tid uint32) *ir.Block {
	// Simple search (could be cached in Temporary)
	for _, b := range f.Blocks {
		for _, p := range b.Phis {
			if p.To.IsTmp() && p.To.Val == tid {
				return b
			}
		}
		for _, ins := range b.Ins {
			if ins.To.IsTmp() && ins.To.Val == tid {
				return b
			}
		}
	}
	return nil
}
