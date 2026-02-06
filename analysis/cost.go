package analysis

import (
	"github.com/ugurkorkmaz/qbe-go/ir"
)

// FillCost computes spill costs and usage statistics for each temporary.
func FillCost(f *ir.Function) {
	for i := range f.Temps {
		t := &f.Temps[i]
		t.Cost = 0
		t.NDef = 0
		t.NUse = 0
	}

	for _, b := range f.Blocks {
		loop := b.Loop

		// Handle Phis
		for _, p := range b.Phis {
			if p.To.IsTmp() {
				tmpUse(f, p.To, false, 0)
			}
			for i, arg := range p.Args {
				// Phi arguments contribute to cost based on predecessor's loop depth
				argLoop := p.Blks[i].Loop
				tmpUse(f, arg, true, argLoop)
			}
		}

		// Handle Instructions
		for i := range b.Ins {
			ins := &b.Ins[i]
			if ins.To.IsTmp() {
				tmpUse(f, ins.To, false, loop)
			}
			for _, arg := range ins.Arg {
				tmpUse(f, arg, true, loop)
			}
		}

		// Handle Jump
		if b.Jmp.Arg.IsTmp() {
			tmpUse(f, b.Jmp.Arg, true, loop)
		}
	}
}

func tmpUse(f *ir.Function, r ir.Ref, isUse bool, loop uint32) {
	if !r.IsTmp() {
		return
	}
	t := &f.Temps[r.Val]
	if isUse {
		t.NUse++
	} else {
		t.NDef++
	}
	t.Cost += loop
}
