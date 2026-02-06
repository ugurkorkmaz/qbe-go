package analysis

import (
	"github.com/ugurkorkmaz/qbe-go/ir"
	"github.com/ugurkorkmaz/qbe-go/util"
)

// Liveness performs liveness analysis on the function.
func Liveness(f *ir.Function) {
	// 1. Initialize bitsets
	for _, b := range f.Blocks {
		b.In = util.NewBitSet(f.NTmp)
		b.Out = util.NewBitSet(f.NTmp)
		b.Gen = util.NewBitSet(f.NTmp)
		b.Kill = util.NewBitSet(f.NTmp)

		for _, ins := range b.Ins {
			for _, arg := range ins.Arg {
				if arg.IsTmp() && !b.Kill.Has(arg.Val) {
					b.Gen.Set(arg.Val)
				}
			}
			if ins.To.IsTmp() {
				b.Kill.Set(ins.To.Val)
			}
		}

		if b.Jmp.Arg.IsTmp() && !b.Kill.Has(b.Jmp.Arg.Val) {
			b.Gen.Set(b.Jmp.Arg.Val)
		}
	}

	// 2. Iteratively solve dataflow equations
	changed := true
	for changed {
		changed = false
		for i := len(f.Blocks) - 1; i >= 0; i-- {
			b := f.Blocks[i]

			newOut := util.NewBitSet(f.NTmp)
			if b.S1 != nil {
				liveOn(newOut, b, b.S1)
			}
			if b.S2 != nil {
				liveOn(newOut, b, b.S2)
			}

			if b.Out.Union(newOut) {
				changed = true
			}

			newIn := util.NewBitSet(f.NTmp)
			newIn.Copy(b.Out)
			newIn.Diff(b.Kill)
			newIn.Union(b.Gen)

			if b.In.Union(newIn) {
				changed = true
			}
		}
	}
}

func liveOn(v util.BitSet, b *ir.Block, s *ir.Block) {
	v.Union(s.In)

	for _, p := range s.Phis {
		if p.To.IsTmp() {
			v.Clear(p.To.Val)
		}
	}

	for _, p := range s.Phis {
		for i, arg := range p.Args {
			if p.Blks[i] == b && arg.IsTmp() {
				v.Set(arg.Val)
			}
		}
	}
}
