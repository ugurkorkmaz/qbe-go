package codegen

import (
	"github.com/ugurkorkmaz/qbe-go/arch"
	"github.com/ugurkorkmaz/qbe-go/ir"
	"github.com/ugurkorkmaz/qbe-go/util"
)

// callerSavedGPR: x0-x17 are caller-saved on ARM64 (AArch64 AAPCS64).
var callerSavedGPR = func() []int {
	s := make([]int, 18)
	for i := range s {
		s[i] = i
	}
	return s
}()

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

// findLiveAcrossCall returns the set of temp IDs that are live across at least
// one call instruction in block b (i.e. defined/live-in before a call and used
// after it).  These temps must land in callee-saved registers so they survive
// the call.
func (ra *RegAllocator) findLiveAcrossCall(b *ir.Block) map[uint32]bool {
	result := make(map[uint32]bool)

	for callIdx, ins := range b.Ins {
		if ins.Op != ir.Ocall {
			continue
		}

		// Collect temps used after this call.
		usedAfter := make(map[uint32]bool)
		for j := callIdx + 1; j < len(b.Ins); j++ {
			for n := 0; n < 2; n++ {
				if b.Ins[j].Arg[n].IsTmp() {
					usedAfter[b.Ins[j].Arg[n].Val] = true
				}
			}
		}
		if b.Jmp.Arg.IsTmp() {
			usedAfter[b.Jmp.Arg.Val] = true
		}
		for t := uint32(0); t < ra.F.NTmp; t++ {
			if b.Out.Has(t) {
				usedAfter[t] = true
			}
		}

		// A temp is live-across-call if it is also defined/live-in before the call.
		for t := range usedAfter {
			definedBefore := b.In.Has(t)
			if !definedBefore {
				for j := 0; j < callIdx; j++ {
					if b.Ins[j].To.IsTmp() && b.Ins[j].To.Val == t {
						definedBefore = true
						break
					}
				}
			}
			if definedBefore {
				result[t] = true
			}
		}
	}
	return result
}

// Allocate performs Linear Scan Register Allocation on the function.
func (ra *RegAllocator) Allocate() {
	for _, b := range ra.F.Blocks {
		ra.allocBlock(b)
	}
}

func (ra *RegAllocator) allocBlock(b *ir.Block) {
	liveAcrossCall := ra.findLiveAcrossCall(b)

	state := &RegState{
		TtoR: make(map[uint32]int),
		RtoT: make(map[int]uint32),
		Live: util.NewBitSet(ra.F.NTmp + 64),
	}

	// Seed live-out temporaries with registers, respecting cross-block assignments.
	for t := uint32(0); t < ra.F.NTmp; t++ {
		if b.Out.Has(t) {
			if globalReg, found := ra.GlobalTR[t]; found {
				ra.takeReg(state, t, globalReg)
			} else {
				ra.assign(state, t, -1, false)
			}
		}
	}

	// Pre-assign live-in temps that cross a call to callee-saved registers so
	// the call does not clobber them.  Record any register remapping so we can
	// emit a fixup move at the top of the block.
	type remap struct{ tid uint32; old, new_ int }
	var remaps []remap

	for t := uint32(0); t < ra.F.NTmp; t++ {
		if !liveAcrossCall[t] || !b.In.Has(t) {
			continue
		}
		if _, already := state.TtoR[t]; already {
			continue // already assigned from b.Out seeding
		}
		oldReg := -1
		if gr, found := ra.GlobalTR[t]; found {
			oldReg = gr
		}
		newReg := ra.assign(state, t, -1, true /* callee-saved */)
		if oldReg != -1 && oldReg != newReg {
			remaps = append(remaps, remap{t, oldReg, newReg})
		}
		// Update GlobalTR so successor blocks use the callee-saved reg too.
		ra.GlobalTR[t] = newReg
	}

	// Process jump argument before the backward scan (Jjnz arg is last in execution order).
	if b.Jmp.Type == ir.Jjnz && b.Jmp.Arg.IsTmp() {
		tid := b.Jmp.Arg.Val
		reg, ok := state.TtoR[tid]
		if !ok {
			if globalReg, found := ra.GlobalTR[tid]; found {
				reg = ra.takeReg(state, tid, globalReg)
			} else {
				reg = ra.assign(state, tid, -1, false)
			}
		}
		b.Jmp.Arg = ir.PhysicalReg(reg)
	}

	// Process instructions backwards.
	for i := len(b.Ins) - 1; i >= 0; i-- {
		ins := &b.Ins[i]

		// Handle Definition: the result temp is no longer live before this point.
		if ins.To.IsTmp() {
			tid := ins.To.Val
			if reg, ok := state.TtoR[tid]; ok {
				ins.To = ir.PhysicalReg(reg)
				ra.freeReg(state, tid, reg)
			} else {
				// Coalescing: copy from a free physical reg → reuse that reg.
				forcedReg := -1
				if ins.Op == ir.Ocopy && ins.Arg[0].Kind == ir.RReg {
					srcReg := int(ins.Arg[0].Val)
					if !state.Live.Has(uint32(srcReg)) {
						forcedReg = srcReg
					}
				}
				var reg int
				if forcedReg != -1 {
					reg = ra.takeReg(state, tid, forcedReg)
				} else {
					reg = ra.assign(state, tid, -1, false)
				}
				ins.To = ir.PhysicalReg(reg)
				ra.freeReg(state, tid, reg)
			}
		} else if ins.To.Kind == ir.RReg {
			ra.freeReg(state, 0, int(ins.To.Val))
		}

		// Handle Uses: source temps become live before this instruction.
		for n := 0; n < 2; n++ {
			arg := &ins.Arg[n]
			if arg.IsTmp() {
				tid := arg.Val
				reg, ok := state.TtoR[tid]
				if !ok {
					if globalReg, found := ra.GlobalTR[tid]; found {
						reg = ra.takeReg(state, tid, globalReg)
					} else {
						fav := -1
						if ins.Op == ir.Ocopy && ins.To.Kind == ir.RReg {
							fav = int(ins.To.Val)
						}
						reg = ra.assign(state, tid, fav, false)
					}
				}
				*arg = ir.PhysicalReg(reg)
			} else if arg.Kind == ir.RReg {
				ra.takeReg(state, 0, int(arg.Val))
			}
		}
	}

	// Prepend fixup moves for any live-in temps remapped to callee-saved registers.
	if len(remaps) > 0 {
		prefix := make([]ir.Instruction, 0, len(remaps))
		for _, r := range remaps {
			cls := ra.F.Temps[r.tid].Cls
			prefix = append(prefix, ir.Instruction{
				Op:  ir.Ocopy,
				Cls: cls,
				To:  ir.PhysicalReg(r.new_),
				Arg: [2]ir.Ref{ir.PhysicalReg(r.old), ir.Undef},
			})
		}
		b.Ins = append(prefix, b.Ins...)
	}
}

// assign picks a free register for temp tid.
// fav is a hint register (-1 = no preference).
// calleeSavedOnly forces selection from the callee-saved pool (x19-x28).
func (ra *RegAllocator) assign(state *RegState, tid uint32, fav int, calleeSavedOnly bool) int {
	cls := ra.F.Temps[tid].Cls
	var regs []int

	if cls.IsFloat() {
		for r := 0; r < 32; r++ {
			regs = append(regs, ra.Target.FPR0()+r)
		}
	} else if calleeSavedOnly {
		// x19-x28 are callee-saved on ARM64.
		regs = []int{19, 20, 21, 22, 23, 24, 25, 26, 27, 28}
	} else {
		// Caller-saved first (0-11), then callee-saved (19-28) as spill targets.
		regs = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28}
	}

	if fav != -1 && !state.Live.Has(uint32(fav)) {
		return ra.takeReg(state, tid, fav)
	}

	for _, r := range regs {
		if !state.Live.Has(uint32(r)) {
			return ra.takeReg(state, tid, r)
		}
	}

	panic("Register pressure too high even after Spill pass!")
}

func (ra *RegAllocator) takeReg(state *RegState, tid uint32, reg int) int {
	state.TtoR[tid] = reg
	state.RtoT[reg] = tid
	state.Live.Set(uint32(reg))
	if tid != 0 {
		ra.GlobalTR[tid] = reg
		// Track callee-saved register usage for prologue/epilogue emission.
		if reg >= 19 && reg <= 28 {
			ra.recordCalleeSaved(reg)
		}
	}
	return reg
}

func (ra *RegAllocator) recordCalleeSaved(reg int) {
	for _, r := range ra.F.CalleeSaved {
		if r == reg {
			return
		}
	}
	ra.F.CalleeSaved = append(ra.F.CalleeSaved, reg)
}

func (ra *RegAllocator) freeReg(state *RegState, tid uint32, reg int) {
	delete(state.TtoR, tid)
	delete(state.RtoT, reg)
	state.Live.Clear(uint32(reg))
}
