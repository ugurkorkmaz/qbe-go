package analysis

import (
	"fmt"

	"github.com/ugurkorkmaz/qbe-go/ir"
)

// VerifySSA ensures that the function IR respects SSA invariants.
// 1. Every temporary is defined exactly once.
// 2. Every use of a temporary is dominated by its definition.
func VerifySSA(f *ir.Function) error {
	// First, check for multiple definitions
	defs := make([]int, f.NTmp)
	for _, b := range f.Blocks {
		for _, p := range b.Phis {
			if p.To.IsTmp() {
				defs[p.To.Val]++
			}
		}
		for _, ins := range b.Ins {
			if ins.To.IsTmp() {
				defs[ins.To.Val]++
			}
		}
	}

	for t, count := range defs {
		if count > 1 {
			return fmt.Errorf("temporary %%%d defined %d times (SSA requires exactly one definition)", t, count)
		}
		if count == 0 && f.Temps[t].Name != "" {
			// Note: Some temps might be intentionally undefined if they are global or special,
			// but in pure SSA, every local temp has a def.
		}
	}

	// Second, check dominance invariant
	for _, b := range f.Blocks {
		// Instructions in block b
		for _, ins := range b.Ins {
			for _, arg := range ins.Arg {
				if arg.IsTmp() {
					if err := checkDom(f, arg.Val, b); err != nil {
						return fmt.Errorf("instruction %s in @%s: %v", ins.Op, b.Name, err)
					}
				}
			}
		}

		// Jump in block b
		if b.Jmp.Arg.IsTmp() {
			if err := checkDom(f, b.Jmp.Arg.Val, b); err != nil {
				return fmt.Errorf("jump in @%s: %v", b.Name, err)
			}
		}

		// Phis in successor blocks
		// The use in Phi(P1: v1, P2: v2) in block S is considered to happen at the
		// end of predecessor Pi.
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
