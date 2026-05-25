package codegen

import (
	"github.com/ugurkorkmaz/qbe-go/arch"
	"github.com/ugurkorkmaz/qbe-go/ir"
	"github.com/ugurkorkmaz/qbe-go/util"
)

type RegAllocator struct {
	F        *ir.Function
	Target   arch.Target
	GlobalTR map[uint32]int
}

func NewRegAllocator(f *ir.Function, t arch.Target) *RegAllocator {
	return &RegAllocator{
		F:        f,
		Target:   t,
		GlobalTR: make(map[uint32]int),
	}
}

type RegState struct {
	TtoR map[uint32]int
	RtoT map[int]uint32
	Live util.BitSet
}

func (ra *RegAllocator) Allocate() {
	liveAcrossCalls := ra.findGlobalLiveAcrossCalls()
	for i := 0; i < 2; i++ {
		for _, b := range ra.F.Blocks {
			ra.allocBlock(b, liveAcrossCalls, true)
		}
	}
	ra.F.CalleeSaved = nil
	for _, b := range ra.F.Blocks {
		ra.allocBlock(b, liveAcrossCalls, false)
	}
}

func (ra *RegAllocator) findGlobalLiveAcrossCalls() map[uint32]bool {
	global := make(map[uint32]bool)
	for _, b := range ra.F.Blocks {
		for callIdx, ins := range b.Ins {
			if ins.Op != ir.Ocall { continue }
			usedAfter := make(map[uint32]bool)
			for j := callIdx + 1; j < len(b.Ins); j++ {
				for n := 0; n < 3; n++ {
					if b.Ins[j].Arg[n].IsTmp() { usedAfter[b.Ins[j].Arg[n].Val] = true }
				}
			}
			if b.Jmp.Arg.IsTmp() { usedAfter[b.Jmp.Arg.Val] = true }
			for t := uint32(0); t < ra.F.NTmp; t++ {
				if b.Out.Has(t) { usedAfter[t] = true }
			}
			for t := range usedAfter {
				global[t] = true
			}
		}
	}
	return global
}

func (ra *RegAllocator) allocBlock(b *ir.Block, globalLiveAcrossCalls map[uint32]bool, dryRun bool) {
	state := &RegState{
		TtoR: make(map[uint32]int),
		RtoT: make(map[int]uint32),
		Live: util.NewBitSet(ra.F.NTmp + 64),
	}
	state.Live.Reset()

	type remap struct { tid uint32; old, new int }
	var remaps []remap

	// 1. Seed with GlobalTR for live-in temps
	for t := uint32(0); t < ra.F.NTmp; t++ {
		if b.In.Has(t) {
			favReg := -1
			if gr, found := ra.GlobalTR[t]; found { favReg = gr }
			reg := ra.assign(state, t, favReg, globalLiveAcrossCalls[t], -1)
			if !dryRun && favReg != -1 && favReg != reg {
				remaps = append(remaps, remap{t, favReg, reg})
			}
			ra.GlobalTR[t] = reg
		}
	}

	// 2. Pre-assign live-out temps so we don't pick their registers for other uses
	for t := uint32(0); t < ra.F.NTmp; t++ {
		if b.Out.Has(t) && !b.In.Has(t) {
			// This temp is defined in this block and live-out. 
			// Check if we already have a suggestion for it from successors.
			if favReg, found := ra.GlobalTR[t]; found {
				if !state.Live.Has(uint32(favReg)) {
					ra.takeReg(state, t, favReg)
				}
			}
		}
	}

	// Process Jump argument
	if b.Jmp.Type == ir.Jjnz && b.Jmp.Arg.IsTmp() {
		tid := b.Jmp.Arg.Val
		reg, ok := state.TtoR[tid]
		if !ok { reg = ra.assign(state, tid, -1, globalLiveAcrossCalls[tid], -1) }
		if !dryRun { b.Jmp.Arg = ir.PhysicalReg(reg) }
	}

	for i := len(b.Ins) - 1; i >= 0; i-- {
		ins := &b.Ins[i]
		if ins.To.IsTmp() {
			tid := ins.To.Val
			if reg, ok := state.TtoR[tid]; ok {
				if !dryRun { ins.To = ir.PhysicalReg(reg) }
				ra.freeReg(state, tid, reg)
			} else {
				forbidden := ra.getForbidden(ins, state)
				fav := -1
				if ins.Op == ir.Ocopy && ins.Arg[0].Kind == ir.RReg { fav = int(ins.Arg[0].Val) }
				reg := ra.assign(state, tid, fav, globalLiveAcrossCalls[tid], forbidden)
				if !dryRun { ins.To = ir.PhysicalReg(reg) }
				ra.freeReg(state, tid, reg)
			}
		} else if ins.To.Kind == ir.RReg {
			ra.freeReg(state, 0, int(ins.To.Val))
		}
		for n := 0; n < 3; n++ {
			arg := &ins.Arg[n]
			if arg.IsTmp() {
				tid := arg.Val
				reg, ok := state.TtoR[tid]
				if !ok {
					fav := -1
					if ins.Op == ir.Ocopy && ins.To.Kind == ir.RReg { fav = int(ins.To.Val) }
					reg = ra.assign(state, tid, fav, globalLiveAcrossCalls[tid], -1)
				}
				if !dryRun { *arg = ir.PhysicalReg(reg) }
			} else if arg.Kind == ir.RReg {
				ra.takeReg(state, 0, int(arg.Val))
			}
		}
	}

	if !dryRun && len(remaps) > 0 {
		var fixups []ir.Instruction
		for _, r := range remaps {
			fixups = append(fixups, ir.Instruction{
				Op: ir.Ocopy, Cls: ra.F.Temps[r.tid].Cls,
				To: ir.PhysicalReg(r.new), Arg: [3]ir.Ref{ir.PhysicalReg(r.old), ir.Undef, ir.Undef},
			})
		}
		b.Ins = append(fixups, b.Ins...)
	}
	for t := uint32(0); t < ra.F.NTmp; t++ {
		if b.Out.Has(t) {
			if reg, ok := state.TtoR[t]; ok { ra.GlobalTR[t] = reg }
		}
	}
}

func (ra *RegAllocator) getForbidden(ins *ir.Instruction, state *RegState) int {
	for n := 0; n < 3; n++ {
		if ins.Arg[n].IsTmp() {
			if r, ok := state.TtoR[ins.Arg[n].Val]; ok { return r }
		} else if ins.Arg[n].Kind == ir.RReg { return int(ins.Arg[n].Val) }
	}
	return -1
}

func (ra *RegAllocator) assign(state *RegState, tid uint32, fav int, calleeSavedOnly bool, forbidden int) int {
	cls := ra.F.Temps[tid].Cls
	var regs []int
	if cls.IsFloat() {
		for r := 0; r < 32; r++ { regs = append(regs, ra.Target.FPR0()+r) }
	} else {
		// Priority: Callee-saved (19-28), then General (8-15), then Args (0-7)
		regs = []int{19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 8, 9, 10, 11, 12, 13, 14, 15, 0, 1, 2, 3, 4, 5, 6, 7}
	}
	if fav != -1 && fav != forbidden && !state.Live.Has(uint32(fav)) {
		return ra.takeReg(state, tid, fav)
	}
	for _, r := range regs {
		if r != forbidden && !state.Live.Has(uint32(r)) {
			return ra.takeReg(state, tid, r)
		}
	}
	panic("Register pressure too high!")
}

func (ra *RegAllocator) takeReg(state *RegState, tid uint32, reg int) int {
	state.TtoR[tid] = reg
	state.RtoT[reg] = tid
	state.Live.Set(uint32(reg))
	if tid != 0 {
		ra.GlobalTR[tid] = reg
		if reg >= 19 && reg <= 28 { ra.recordCalleeSaved(reg) }
	}
	return reg
}

func (ra *RegAllocator) recordCalleeSaved(reg int) {
	for _, r := range ra.F.CalleeSaved {
		if r == reg { return }
	}
	ra.F.CalleeSaved = append(ra.F.CalleeSaved, reg)
}

func (ra *RegAllocator) freeReg(state *RegState, tid uint32, reg int) {
	delete(state.TtoR, tid)
	delete(state.RtoT, reg)
	state.Live.Clear(uint32(reg))
}
