package codegen

import (
	"sort"

	"github.com/ugurkorkmaz/qbe-go/arch"
	"github.com/ugurkorkmaz/qbe-go/ir"
	"github.com/ugurkorkmaz/qbe-go/util"
)

// Spill performs spill code insertion and register pressure limiting.
// It ensures that at any point in the program, the number of live temporaries
// does not exceed the number of available physical registers (K).
//
// Algorithm:
// 1. Liveness & Pressure Analysis:
//   - Perform a backward scan on instructions.
//   - Track the number of live GPR and FPR temporaries.
//   - If live count > K, select a temporary to spill based on heuristic (currently lowest loop-depth cost).
//   - Mark the temporary as "Spilled" (assign a Slot index).
//
// 2. Rewrite Pass (rewriteSpills):
//   - Insert Ostore instructions after the definition of a spilled temporary.
//   - Insert Oload instructions before the use of a spilled temporary.
//   - This effectively splits the live range of the spilled variable into tiny segments around uses/defs.
func Spill(f *ir.Function, target arch.Target) {
	// We need 2 masks: GPR and FPR
	masks := [2]util.BitSet{
		util.NewBitSet(f.NTmp),
		util.NewBitSet(f.NTmp),
	}
	for i := range f.Temps {
		if f.Temps[i].Cls.IsFloat() {
			masks[1].Set(uint32(i))
		} else {
			masks[0].Set(uint32(i))
		}
	}

	// Process blocks in RPO backwards
	for i := len(f.Blocks) - 1; i >= 0; i-- {
		b := f.Blocks[i]

		// Initial live set at the end of block
		live := util.NewBitSet(f.NTmp)
		live.Copy(b.Out)

		// Limit the initial live set
		limit(f, live, masks[0], target.NGPR())
		limit(f, live, masks[1], target.NFPR())

		// b.Out is updated to reflect only register-assigned temps
		b.Out.Copy(live)

		// Process instructions backwards
		for j := len(b.Ins) - 1; j >= 0; j-- {
			ins := &b.Ins[j]

			// Definition kills liveness
			if ins.To.IsTmp() {
				live.Clear(ins.To.Val)
			}

			// Arguments add to liveness
			for _, arg := range ins.Arg {
				if arg.IsTmp() {
					live.Set(arg.Val)
				}
			}

			// After each instruction, we must ensure pressure is within limits
			limit(f, live, masks[0], target.NGPR())
			limit(f, live, masks[1], target.NFPR())

			// Handle Oalloc instructions by assigning them a permanent slot
			if ins.Op == ir.Oalloc4 || ins.Op == ir.Oalloc8 || ins.Op == ir.Oalloc16 {
				if f.Temps[ins.To.Val].Slot == -1 {
					f.Temps[ins.To.Val].Slot = int(f.NTmp) // Placeholder index for frame calculation
				}
			}
		}

		// b.In is updated
		b.In.Copy(live)
	}

	rewriteSpills(f)
}

func rewriteSpills(f *ir.Function) {
	for _, b := range f.Blocks {
		var newIns []ir.Instruction
		for _, ins := range b.Ins {
			// 1. Reload spilled arguments
			for i := 0; i < 2; i++ {
				arg := ins.Arg[i]
				if arg.IsTmp() && f.Temps[arg.Val].Slot != -1 {
					// This arg is spilled. We must load it into a new short-lived temp.
					slot := f.Temps[arg.Val].Slot
					newTmp := f.NewTmp("reload", f.Temps[arg.Val].Cls)
					// Mark new temp as NOT spilled (it lives in register)
					f.Temps[newTmp.Val].Slot = -1

					// Insert Load: newTmp = Load Slot
					// We use RSlot references which are handled by emit.go.
					loadIns := ir.Instruction{
						Op:  ir.Oload,
						Cls: f.Temps[arg.Val].Cls,
						To:  newTmp,
						Arg: [2]ir.Ref{ir.NewSlot(uint32(slot)), ir.Undef},
					}
					// Handle floating point load if necessary (Oload works for both GPR/FPR in emit.go based on Cls)
					newIns = append(newIns, loadIns)

					// Replace usage
					ins.Arg[i] = newTmp
				}
			}

			// Add the (possibly modified) instruction
			newIns = append(newIns, ins)

			// 2. Store spilled result
			if ins.To.IsTmp() && f.Temps[ins.To.Val].Slot != -1 {
				// This definition is spilled. We must store it to stack.
				// But wait, RA needs to allocate a register for this `ins.To` first.
				// So we keep `ins.To` as/is (it will get a register), and then store that register.
				// Wait, if we keep `ins.To` as the spilled Temp(X), RA will see Temp(X) and say "It's spilled! No register for you!". Panik.
				// So we must REPLACE `ins.To` with a new short-lived Temp(Y).
				// Ins: `Temp(Y) = ...`
				// Store: `Store Temp(Y), Slot(X)`

				origTmp := ins.To
				slot := f.Temps[origTmp.Val].Slot
				newTmp := f.NewTmp("spill", f.Temps[origTmp.Val].Cls)
				f.Temps[newTmp.Val].Slot = -1

				// Update instruction to define new temp
				// We need to modify 'ins' in 'newIns'. Since 'ins' is a copy range var,
				// and we appended it to newIns, we need to modify the last element of newIns.
				newIns[len(newIns)-1].To = newTmp

				// Determine store opcode based on class/size?
				// emit.go supports Ostorew, Ostorel, Ostores, Ostored.
				// Let's pick based on Cls.
				var storeOp ir.Opcode
				switch f.Temps[origTmp.Val].Cls {
				case ir.Kw:
					storeOp = ir.Ostorew
				case ir.Kl:
					storeOp = ir.Ostorel
				case ir.Ks:
					storeOp = ir.Ostores
				case ir.Kd:
					storeOp = ir.Ostored
				default:
					storeOp = ir.Ostorel // Fallback
				}

				storeIns := ir.Instruction{
					Op:  storeOp,
					Cls: ir.Kl, // Store opcode cls usually doesn't matter or is address cls
					Arg: [2]ir.Ref{newTmp, ir.NewSlot(uint32(slot))},
				}
				newIns = append(newIns, storeIns)
			}
		}
		b.Ins = newIns
	}
}

func limit(f *ir.Function, live, mask util.BitSet, k int) {
	inMask := util.NewBitSet(f.NTmp)
	inMask.Copy(live)
	inMask.Intersect(mask)

	count := inMask.Count()
	if count <= uint32(k) {
		return
	}

	// We need to spill 'count - k' temporaries with the lowest cost
	var candidates []uint32
	for t := uint32(0); t < f.NTmp; t++ {
		if inMask.Has(t) {
			candidates = append(candidates, t)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return f.Temps[candidates[i]].Cost < f.Temps[candidates[j]].Cost
	})

	toSpill := count - uint32(k)
	for i := uint32(0); i < toSpill; i++ {
		t := candidates[i]
		live.Clear(t)
		// Mark as spilled
		if f.Temps[t].Slot == -1 {
			f.Temps[t].Slot = 0 // Will be assigned real offset later
		}
	}
}
