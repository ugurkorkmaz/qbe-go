package analysis

import (
	"github.com/ugurkorkmaz/qbe-go/ir"
	"github.com/ugurkorkmaz/qbe-go/util"
)

// SSA performs the Static Single Assignment construction.
func SSA(f *ir.Function) {
	FillPreds(f)
	FillRPO(f)
	FillDom(f)
	FillFron(f)
	Liveness(f)
	FillUse(f)

	insertPhis(f)
	rename(f)
}

func FillUse(f *ir.Function) {
	for i := range f.Temps {
		f.Temps[i].Uses = nil
		f.Temps[i].Def = nil
	}

	for _, b := range f.Blocks {
		for _, p := range b.Phis {
			for _, arg := range p.Args {
				if arg.IsTmp() {
					f.Temps[arg.Val].Uses = append(f.Temps[arg.Val].Uses, ir.Use{
						Kind: ir.UPhi,
						Bid:  b.Id,
						Phi:  p,
					})
				}
			}
		}
		for i := range b.Ins {
			ins := &b.Ins[i]
			if ins.To.IsTmp() {
				f.Temps[ins.To.Val].Def = ins
			}
			for _, arg := range ins.Arg {
				if arg.IsTmp() {
					f.Temps[arg.Val].Uses = append(f.Temps[arg.Val].Uses, ir.Use{
						Kind: ir.UIns,
						Bid:  b.Id,
						Ins:  ins,
					})
				}
			}
		}
		if b.Jmp.Arg.IsTmp() {
			f.Temps[b.Jmp.Arg.Val].Uses = append(f.Temps[b.Jmp.Arg.Val].Uses, ir.Use{
				Kind: ir.UJmp,
				Bid:  b.Id,
			})
		}
	}
}

func insertPhis(f *ir.Function) {
	defs := make([]util.BitSet, f.NTmp)
	for t := uint32(0); t < f.NTmp; t++ {
		defs[t] = util.NewBitSet(f.NBlk)
	}

	for _, b := range f.Blocks {
		for _, ins := range b.Ins {
			if ins.To.IsTmp() {
				defs[ins.To.Val].Set(b.Id)
			}
		}
		for _, p := range b.Phis {
			if p.To.IsTmp() {
				defs[p.To.Val].Set(b.Id)
			}
		}
	}

	for t := uint32(0); t < f.NTmp; t++ {
		worklist := []uint32{}
		for bId := uint32(0); bId < f.NBlk; bId++ {
			if defs[t].Has(bId) {
				worklist = append(worklist, bId)
			}
		}

		addedPhi := util.NewBitSet(f.NBlk)
		for len(worklist) > 0 {
			nId := worklist[0]
			worklist = worklist[1:]

			n := f.Blocks[nId]
			for _, df := range n.Fron {
				if !addedPhi.Has(df.Id) {
					if df.In.Has(t) {
						df.Phis = append(df.Phis, &ir.Phi{
							To:  ir.NewTmp(t),
							Cls: f.Temps[t].Cls,
						})
						addedPhi.Set(df.Id)
						if !defs[t].Has(df.Id) {
							worklist = append(worklist, df.Id)
						}
					}
				}
			}
		}
	}
}

func rename(f *ir.Function) {
	stacks := make([][]ir.Ref, f.NTmp)
	newToOld := make(map[uint32]uint32)
	for t := uint32(0); t < f.NTmp; t++ {
		newToOld[t] = t
	}

	var walk func(b *ir.Block)
	walk = func(b *ir.Block) {
		origHeights := make([]int, f.NTmp)
		for i := range stacks {
			origHeights[i] = len(stacks[i])
		}

		for _, p := range b.Phis {
			oldReg := newToOld[p.To.Val]
			newReg := f.NewTmp(f.Temps[oldReg].Name, p.Cls)
			p.To = newReg
			newToOld[newReg.Val] = oldReg
			stacks[oldReg] = append(stacks[oldReg], newReg)
		}

		for i := range b.Ins {
			ins := &b.Ins[i]
			for n := 0; n < 3; n++ {
				if ins.Arg[n].IsTmp() {
					tid := newToOld[ins.Arg[n].Val]
					if len(stacks[tid]) > 0 {
						ins.Arg[n] = stacks[tid][len(stacks[tid])-1]
					}
				}
			}
			if ins.To.IsTmp() {
				oldReg := newToOld[ins.To.Val]
				newReg := f.NewTmp(f.Temps[oldReg].Name, ins.Cls)
				ins.To = newReg
				newToOld[newReg.Val] = oldReg
				stacks[oldReg] = append(stacks[oldReg], newReg)
			}
		}

		if b.Jmp.Arg.IsTmp() {
			tid := newToOld[b.Jmp.Arg.Val]
			if len(stacks[tid]) > 0 {
				b.Jmp.Arg = stacks[tid][len(stacks[tid])-1]
			}
		}

		// Update Succ Phis
		succs := []*ir.Block{b.S1}
		if b.S2 != nil && b.S2 != b.S1 {
			succs = append(succs, b.S2)
		}
		for _, s := range succs {
			if s == nil {
				continue
			}
			for _, p := range s.Phis {
				origTid := newToOld[p.To.Val]
				if len(stacks[origTid]) > 0 {
					p.Args = append(p.Args, stacks[origTid][len(stacks[origTid])-1])
					p.Blks = append(p.Blks, b)
				}
			}
		}

		for s := b.Dom; s != nil; s = s.DLink {
			walk(s)
		}

		for i := range stacks {
			stacks[i] = stacks[i][:origHeights[i]]
		}
	}

	walk(f.Start)
}
