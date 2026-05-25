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

// Allocate performs Linear Scan Register Allocation on the function.
// It assigns physical registers to SSA temporaries, minimizing spill costs.
//
// Algorithm Overview:
//  1. Liveness Analysis: Determine which variables are live at each instruction.
//     (Handled by analysis.Liveness before this pass, but we track sub-block liveness here).
//  2. Backward Scan: We iterate instructions from bottom to top.
//     - This allows us to see "future uses" first (which are current uses in backward scan).
//     - We maintain a set of "Live" physical registers.
//     - When we see a Use (Arg), we allocate a register for it.
//     - When we see a Def (To), we free that register (it's no longer needed before this point).
//
// Optimizations:
//   - Coalescing: If we see `a = copy b` and `b` is assigned to R0, we try to assign `a` to R0 too.
//     This makes the copy instruction `R0 = copy R0`, which is a no-op and removed.
//   - Hinting: If a variable is used as a function argument (e.g. in R0), we try to
//     assign it to R0 throughout its life to avoid moving it.
func (ra *RegAllocator) Allocate() {
	for _, b := range ra.F.Blocks {
		ra.allocBlock(b)
	}
}

// allocBlock processes a single basic block in reverse order.
// Note: We assume SSA properties involved (Phi nodes) are handled or lowered before this.
// QBE handles Phis by inserting copies at the end of predecessor blocks.
func (ra *RegAllocator) allocBlock(b *ir.Block) {
	// Initialize the state with liveness information at the end of the block.
	// b.Out contains temporaries that are live-out (needed by successor blocks).
	state := &RegState{
		TtoR: make(map[uint32]int),
		RtoT: make(map[int]uint32),
		Live: util.NewBitSet(ra.F.NTmp + 64),
	}

	// Seed live-out temporaries with registers (pre-rename IDs may be stale, best-effort).
	for t := uint32(0); t < ra.F.NTmp; t++ {
		if b.Out.Has(t) {
			ra.assign(state, t, -1)
		}
	}

	// Process jump argument before the backward scan (jump is last in execution order).
	if b.Jmp.Type == ir.Jjnz && b.Jmp.Arg.IsTmp() {
		tid := b.Jmp.Arg.Val
		reg, ok := state.TtoR[tid]
		if !ok {
			if globalReg, found := ra.GlobalTR[tid]; found {
				reg = ra.takeReg(state, tid, globalReg)
			} else {
				reg = ra.assign(state, tid, -1)
			}
		}
		b.Jmp.Arg = ir.PhysicalReg(reg)
	}

	// 2. Process instructions backwards.
	for i := len(b.Ins) - 1; i >= 0; i-- {
		ins := &b.Ins[i]

		// Handle Definition: The result temporary 'To' is no longer live before this.
		if ins.To.IsTmp() {
			tid := ins.To.Val
			if reg, ok := state.TtoR[tid]; ok {
				ins.To = ir.PhysicalReg(reg)
				ra.freeReg(state, tid, reg)
			} else {
				// Coalescing optimization:
				// If this is a copy from a physical register (Rsrc), and Rsrc becomes dead here,
				// use Rsrc for 'To' (Tdst).
				// This turns "Tdst = copy Rsrc" into "Rsrc = copy Rsrc" which is eliminated.
				forcedReg := -1
				if ins.Op == ir.Ocopy && ins.Arg[0].Kind == ir.RReg {
					srcReg := int(ins.Arg[0].Val)
					// Check if srcReg is NOT live after this instruction (backward scan logic).
					// In backward scan, 'freeReg' marks 'To' as dead. The 'Uses' loops below mark 'Arg' as live.
					// So if we are here, we haven't processed Uses yet.
					// We need to look ahead (which is 'before' in execution flow) or check current liveness.
					// Actually, simplest check: if srcReg is NOT in state.Live, it means it's not needed by following instructions.
					if !state.Live.Has(uint32(srcReg)) {
						forcedReg = srcReg
					}
				}

				var reg int
				if forcedReg != -1 {
					reg = ra.takeReg(state, tid, forcedReg)
				} else {
					reg = ra.assign(state, tid, -1) // -1 as no specific fav for 'To'
				}
				ins.To = ir.PhysicalReg(reg)
				ra.freeReg(state, tid, reg)
			}
		} else if ins.To.Kind == ir.RReg {
			// Physical register defined - mark it free for preceding code
			ra.freeReg(state, 0 /* unused */, int(ins.To.Val))
		}

		// Handle Uses: The source temporaries 'Arg' become live before this.
		for n := 0; n < 2; n++ {
			arg := &ins.Arg[n]
			if arg.IsTmp() {
				tid := arg.Val
				reg, ok := state.TtoR[tid]
				if !ok {
					// Respect cross-block assignment: if this tmp was assigned in
					// a predecessor block, use the same register for consistency.
					if globalReg, found := ra.GlobalTR[tid]; found {
						reg = ra.takeReg(state, tid, globalReg)
					} else {
						fav := -1
						// HINT: prefer the destination physical register for coalescing,
						// but only when tmp has no prior global assignment.
						if ins.Op == ir.Ocopy && ins.To.Kind == ir.RReg {
							fav = int(ins.To.Val)
						}
						reg = ra.assign(state, tid, fav)
					}
				}
				*arg = ir.PhysicalReg(reg)
			} else if arg.Kind == ir.RReg {
				// Physical register used - mark it busy
				ra.takeReg(state, 0 /* virtual placeholder */, int(arg.Val))
			}
		}
	}
}

func (ra *RegAllocator) assign(state *RegState, tid uint32, fav int) int {
	// Class-based register selection
	cls := ra.F.Temps[tid].Cls
	var regs []int

	if cls.IsFloat() {
		// V0-V7 are parameters, V8-V15 are callee-saved, V16-V31 are callers
		// For simplicity, let's just use a usable range
		for r := 0; r < 32; r++ {
			regs = append(regs, ra.Target.FPR0()+r)
		}
	} else {
		// GPR typical order: RDI, RSI, RDX, RCX, R8, R9, R10, R11, RAX...
		// On ARM64, these map to different numbers, but let's use Target's GPR range
		// Prefer low registers (0, 1...) to increase chance of coalescing with parameters
		gprs := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
		regs = gprs
	}

	if fav != -1 {
		// Aggressively try to use the favorite register.
		// If it's available, take it immediately.
		if !state.Live.Has(uint32(fav)) {
			return ra.takeReg(state, tid, fav)
		}
		// If it's busy, checks if it's busy because of the SAME value (coalescing opportunity)
		// This requires more complex tracking, so for now we stick to availability check.
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
	}
	return reg
}

func (ra *RegAllocator) freeReg(state *RegState, tid uint32, reg int) {
	delete(state.TtoR, tid)
	delete(state.RtoT, reg)
	state.Live.Clear(uint32(reg))
}
