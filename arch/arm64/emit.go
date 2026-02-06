package arm64

import (
	"fmt"

	"github.com/ugurkorkmaz/qbe-go/ir"
)

func (t *ARM64Target) Emit(f *ir.Function, globals []string) error {
	// Print function label
	fmt.Printf(".text\n")
	// fmt.Printf(".balign 4\n")
	if f.Exported {
		fmt.Printf(".globl _%s\n", f.Name)
	}
	fmt.Printf(".p2align 2\n") // 4-byte alignment
	fmt.Printf("_%s:\n", f.Name)

	// Emit Prologue
	// Always save FP and LR, and update FP.
	// This creates a 16-byte frame record at the bottom of the stack.
	fmt.Printf("\tstp x29, x30, [sp, #-16]!\n")
	fmt.Printf("\tmov x29, sp\n")

	frameSize := t.calculateFrame(f)
	if frameSize > 0 {
		t.emitFrameAlloc(frameSize)
	}
	// Note: emitFrameAlloc will bump SP further down if locals exist.

	t.CurrentGlobals = globals

	for _, b := range f.Blocks {
		lblPrefix := ".L"
		if t.Apple {
			lblPrefix = "L"
		}
		// Use function name to disambiguate local labels between functions
		fmt.Printf("%s%s_%d:\t// @%s\n", lblPrefix, f.Name, b.Id, b.Name)

		for _, ins := range b.Ins {
			t.emitIns(ins, f)
		}
		t.emitJump(b, f)
	}

	// Remove hacky data section emit. We will handle data separately.

	return nil
}

func (t *ARM64Target) calculateFrame(f *ir.Function) int {
	maxSlot := -1
	for _, tmp := range f.Temps {
		if tmp.Slot > maxSlot {
			maxSlot = tmp.Slot
		}
	}
	if maxSlot == -1 {
		return 0
	}
	return (maxSlot + 1) * 8
}

func (t *ARM64Target) emitFrameAlloc(size int) {
	size = (size + 15) &^ 15 // 16-byte alignment
	if size <= 4095 {
		fmt.Printf("\tsub sp, sp, #%d\n", size)
	} else {
		fmt.Printf("\tmov x16, #%d\n", size&0xffff)
		if size > 0xffff {
			fmt.Printf("\tmovk x16, #%d, lsl #16\n", (size>>16)&0xffff)
		}
		fmt.Printf("\tsub sp, sp, x16\n")
	}
}

func (t *ARM64Target) emitJump(b *ir.Block, f *ir.Function) {
	lblPrefix := ".L"
	if t.Apple {
		lblPrefix = "L"
	}

	switch b.Jmp.Type {
	case ir.Jjmp:
		if b.S1 != nil {
			fmt.Printf("\tb %s%s_%d\n", lblPrefix, f.Name, b.S1.Id)
		} else {
			fmt.Printf("\tmov sp, x29\n")
			fmt.Printf("\tldp x29, x30, [sp], #16\n")
			fmt.Printf("\tret\n")
		}
	case ir.Jjnz:
		fmt.Printf("\tcbnz %s, %s%s_%d\n", t.formatRef(b.Jmp.Arg, ir.Kw, f), lblPrefix, f.Name, b.S1.Id)
		fmt.Printf("\tb %s%s_%d\n", lblPrefix, f.Name, b.S2.Id)
	}
}

func (t *ARM64Target) emitIns(ins ir.Instruction, f *ir.Function) {
	switch ins.Op {
	case ir.Onop:
		return
	case ir.Oadd:
		t.emitBinop("add", ins, f)
	case ir.Osub:
		t.emitBinop("sub", ins, f)
	case ir.Omul:
		t.emitBinop("mul", ins, f)
	case ir.Odiv:
		t.emitBinop("div", ins, f)
	case ir.Oudiv:
		t.emitBinop("udiv", ins, f)
	case ir.Orem, ir.Ourem:
		t.emitModulo(ins, f)
	case ir.Oand:
		t.emitBinop("and", ins, f)
	case ir.Oor:
		t.emitBinop("orr", ins, f)
	case ir.Oxor:
		t.emitBinop("eor", ins, f)
	case ir.Osar:
		t.emitBinop("asr", ins, f)
	case ir.Oshr:
		t.emitBinop("lsr", ins, f)
	case ir.Oshl:
		t.emitBinop("lsl", ins, f)

	case ir.Ocopy:
		dst := t.formatRef(ins.To, ins.Cls, f)
		srcRef := ins.Arg[0]

		if srcRef.Kind == ir.RSym {
			// Special handling for global symbols: emit address calculation
			symName := t.formatRef(srcRef, ins.Cls, f)
			// Small code model (within +/- 4GB? No, within same section +/- 1MB uses adr)
			// But globals are usually external or data. Use adrp + add for full 4GB relative reach.
			// However, adrp requires page-aligned symbol.
			// Let's use adrp/add sequence which is standard PIC.
			fmt.Printf("\tadrp %s, %s@PAGE\n", dst, symName)
			fmt.Printf("\tadd %s, %s, %s@PAGEOFF\n", dst, dst, symName)
			return
		}

		if srcRef.Kind == ir.RCon || srcRef.Kind == ir.RInt {
			val := uint64(srcRef.Val)
			if srcRef.Kind == ir.RCon {
				val = f.Constants[srcRef.Val].Val
			}
			if ins.Cls.IsFloat() {
				tmpGPR := "x16"
				if ins.Cls == ir.Ks {
					tmpGPR = "w16"
				}
				t.emitMoveImm("x16", val, true)
				fmt.Printf("\tfmov %s, %s\n", dst, tmpGPR)
			} else {
				t.emitMoveImm(dst, val, ins.Cls == ir.Kl)
			}
		} else {
			src := t.formatRef(srcRef, ins.Cls, f)
			if dst != src {
				cmd := "mov"
				if ins.Cls.IsFloat() {
					cmd = "fmov"
				}
				fmt.Printf("\t%s %s, %s\n", cmd, dst, src)
			}
		}

	case ir.Oload:
		fmt.Printf("\tldr %s, [%s]\n", t.formatRef(ins.To, ins.Cls, f), t.formatRef(ins.Arg[0], ir.Kl, f))
	case ir.Ostoreb:
		fmt.Printf("\tstrb %s, [%s]\n", t.formatRef(ins.Arg[0], ir.Kw, f), t.formatRef(ins.Arg[1], ir.Kl, f))
	case ir.Ostoreh:
		fmt.Printf("\tstrh %s, [%s]\n", t.formatRef(ins.Arg[0], ir.Kw, f), t.formatRef(ins.Arg[1], ir.Kl, f))
	case ir.Ostorew:
		fmt.Printf("\tstr %s, [%s]\n", t.formatRef(ins.Arg[0], ir.Kw, f), t.formatRef(ins.Arg[1], ir.Kl, f))
	case ir.Ostorel:
		fmt.Printf("\tstr %s, [%s]\n", t.formatRef(ins.Arg[0], ir.Kl, f), t.formatRef(ins.Arg[1], ir.Kl, f))
	case ir.Ostores, ir.Ostored:
		fmt.Printf("\tstr %s, [%s]\n", t.formatRef(ins.Arg[0], ins.Cls, f), t.formatRef(ins.Arg[1], ir.Kl, f))

	case ir.Oceqw, ir.Oceql, ir.Oceqs, ir.Oceqd:
		t.emitCmp("eq", ins, f)
	case ir.Ocnew, ir.Ocnel, ir.Ocnes, ir.Ocned:
		t.emitCmp("ne", ins, f)
	case ir.Ocsltw, ir.Ocsltl, ir.Oclts, ir.Ocltd:
		t.emitCmp("lt", ins, f)
	case ir.Ocslew, ir.Ocslel, ir.Ocles, ir.Ocled:
		t.emitCmp("le", ins, f)
	case ir.Ocsgtw, ir.Ocsgtl, ir.Ocgts, ir.Ocgtd:
		t.emitCmp("gt", ins, f)
	case ir.Ocsgew, ir.Ocsgel, ir.Ocges, ir.Ocged:
		t.emitCmp("ge", ins, f)

	case ir.Ostosi, ir.Odtosi, ir.Ostoui, ir.Odtoui:
		dst := t.formatRef(ins.To, ins.Cls, f)
		srcCls := ir.Ks
		if ins.Op == ir.Odtosi || ins.Op == ir.Odtoui {
			srcCls = ir.Kd
		}
		src := t.formatRef(ins.Arg[0], srcCls, f)
		cmd := "fcvtzs"
		if ins.Op == ir.Ostoui || ins.Op == ir.Odtoui {
			cmd = "fcvtzu"
		}
		fmt.Printf("\t%s %s, %s\n", cmd, dst, src)

	case ir.Oswtof, ir.Osltof, ir.Ouwtof, ir.Oultof:
		dst := t.formatRef(ins.To, ins.Cls, f)
		src := t.formatRef(ins.Arg[0], ir.Kw, f)
		cmd := "scvtf"
		if ins.Op == ir.Ouwtof || ins.Op == ir.Oultof {
			cmd = "ucvtf"
		}
		fmt.Printf("\t%s %s, %s\n", cmd, dst, src)

	case ir.Oexts, ir.Otruncd:
		fmt.Printf("\tfcvt %s, %s\n", t.formatRef(ins.To, ins.Cls, f), t.formatRef(ins.Arg[0], ir.Kx, f))
	case ir.Ocast:
		fmt.Printf("\tfmov %s, %s\n", t.formatRef(ins.To, ins.Cls, f), t.formatRef(ins.Arg[0], ir.Kx, f))

	case ir.Ocall:
		fmt.Printf("\tbl %s\n", t.formatRef(ins.Arg[0], ir.Kl, f))

	case ir.Oalloc4, ir.Oalloc8, ir.Oalloc16:
		offset := (ins.To.Val + 1) * 8
		fmt.Printf("\tadd %s, x29, #%d\n", t.formatRef(ins.To, ir.Kl, f), offset)

	default:
		fmt.Printf("\t// unhandled op %s\n", ins.Op)
	}
}

func (t *ARM64Target) formatRef(r ir.Ref, cls ir.Class, f *ir.Function) string {
	switch r.Kind {
	case ir.RReg:
		if r.Val == 31 {
			return "sp"
		}
		if r.Val >= uint32(V0) {
			if cls == ir.Ks {
				return fmt.Sprintf("s%d", r.Val-uint32(V0))
			}
			return fmt.Sprintf("d%d", r.Val-uint32(V0))
		}
		if cls == ir.Kw {
			return gpr32Names[r.Val]
		}
		return gprNames[r.Val]
	case ir.RCon:
		return fmt.Sprintf("#%d", f.Constants[r.Val].Val)
	case ir.RInt:
		return fmt.Sprintf("#%d", int32(r.Val))
	case ir.RSlot:
		return fmt.Sprintf("[x29, #%d]", (r.Val+1)*8)
	case ir.RSym:
		// Look up in CurrentGlobals if available
		idx := int(r.Val)
		if idx < len(t.CurrentGlobals) {
			return t.CurrentGlobals[idx]
		}
		return fmt.Sprintf("sym_%d", r.Val)
	default:
		return fmt.Sprintf("VREG_%d", r.Val)
	}
}

func (t *ARM64Target) EmitData(d *ir.Data) {
	fmt.Printf(".data\n")
	if d.Exported {
		fmt.Printf(".globl _%s\n", d.Label)
	}
	fmt.Printf(".p2align 3\n")
	fmt.Printf("_%s:\n", d.Label)

	for _, item := range d.Items {
		switch item.Type {
		case "b":
			if item.String != "" {
				// We need to properly quote the string for assembly
				// But Go's %q adds quotes. Assembly expects quoted string too.
				// Example: .asciz "Hello\n"
				fmt.Printf("\t.asciz %q\n", item.String)
			} else {
				fmt.Printf("\t.byte %d\n", item.Value)
			}
		case "h":
			fmt.Printf("\t.hword %d\n", item.Value)
		case "w":
			fmt.Printf("\t.word %d\n", item.Value)
		case "l":
			if item.Label != "" {
				// If label refers to a symbol/global
				fmt.Printf("\t.quad _%s\n", item.Label)
			} else {
				fmt.Printf("\t.quad %d\n", item.Value)
			}
		case "z":
			fmt.Printf("\t.zero %d\n", item.Value)
		}
	}
}
