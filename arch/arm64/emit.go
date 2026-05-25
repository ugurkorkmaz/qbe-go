package arm64

import (
	"fmt"

	"github.com/ugurkorkmaz/qbe-go/ir"
)

func (t *ARM64Target) symPrefix() string {
	if t.Apple {
		return "_"
	}
	return ""
}

func (t *ARM64Target) Emit(f *ir.Function, globals []string) error {
	pfx := t.symPrefix()
	fmt.Fprintf(t.w(), ".text\n")
	if f.Exported {
		fmt.Fprintf(t.w(), ".globl %s%s\n", pfx, f.Name)
	}
	fmt.Fprintf(t.w(), ".p2align 2\n")
	fmt.Fprintf(t.w(), "%s%s:\n", pfx, f.Name)

	// Emit Prologue
	calleeSaveArea := t.calleeSaveAreaSize(f)
	prologueSize := 16 + calleeSaveArea
	fmt.Fprintf(t.w(), "\tstp x29, x30, [sp, #-%d]!\n", prologueSize)
	for i, reg := range f.CalleeSaved {
		fmt.Fprintf(t.w(), "\tstr %s, [sp, #%d]\n", gprNames[reg], 16+i*8)
	}
	fmt.Fprintf(t.w(), "\tmov x29, sp\n")

	frameSize := t.calculateFrame(f)
	if frameSize > 0 {
		t.emitFrameAlloc(frameSize)
	}

	t.CurrentGlobals = globals

	for _, b := range f.Blocks {
		lblPrefix := ".L"
		if t.Apple {
			lblPrefix = "L"
		}
		// Use function name to disambiguate local labels between functions
		fmt.Fprintf(t.w(), "%s%s_%d:\t// @%s\n", lblPrefix, f.Name, b.Id, b.Name)

		for _, ins := range b.Ins {
			t.emitIns(ins, f)
		}
		t.emitJump(b, f)
	}

	// Remove hacky data section emit. We will handle data separately.

	return nil
}

// calleeSaveAreaSize returns the stack bytes needed to save f.CalleeSaved,
// rounded up to 16-byte alignment.
func (t *ARM64Target) calleeSaveAreaSize(f *ir.Function) int {
	n := len(f.CalleeSaved) * 8
	return (n + 15) &^ 15
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
		fmt.Fprintf(t.w(), "\tsub sp, sp, #%d\n", size)
	} else {
		fmt.Fprintf(t.w(), "\tmov x16, #%d\n", size&0xffff)
		if size > 0xffff {
			fmt.Fprintf(t.w(), "\tmovk x16, #%d, lsl #16\n", (size>>16)&0xffff)
		}
		fmt.Fprintf(t.w(), "\tsub sp, sp, x16\n")
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
			fmt.Fprintf(t.w(), "\tb %s%s_%d\n", lblPrefix, f.Name, b.S1.Id)
		} else {
			calleeSaveArea := t.calleeSaveAreaSize(f)
			prologueSize := 16 + calleeSaveArea
			fmt.Fprintf(t.w(), "\tmov sp, x29\n")
			for i, reg := range f.CalleeSaved {
				fmt.Fprintf(t.w(), "\tldr %s, [sp, #%d]\n", gprNames[reg], 16+i*8)
			}
			fmt.Fprintf(t.w(), "\tldp x29, x30, [sp], #%d\n", prologueSize)
			fmt.Fprintf(t.w(), "\tret\n")
		}
	case ir.Jjnz:
		fmt.Fprintf(t.w(), "\tcbnz %s, %s%s_%d\n", t.formatRef(b.Jmp.Arg, ir.Kw, f), lblPrefix, f.Name, b.S1.Id)
		fmt.Fprintf(t.w(), "\tb %s%s_%d\n", lblPrefix, f.Name, b.S2.Id)
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
		if ins.Cls.IsFloat() {
			t.emitBinop("div", ins, f)
		} else {
			t.emitBinop("sdiv", ins, f)
		}
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
			if t.Apple {
				fmt.Fprintf(t.w(), "\tadrp %s, %s@PAGE\n", dst, symName)
				fmt.Fprintf(t.w(), "\tadd %s, %s, %s@PAGEOFF\n", dst, dst, symName)
			} else {
				fmt.Fprintf(t.w(), "\tadrp %s, %s\n", dst, symName)
				fmt.Fprintf(t.w(), "\tadd %s, %s, :lo12:%s\n", dst, dst, symName)
			}
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
				fmt.Fprintf(t.w(), "\tfmov %s, %s\n", dst, tmpGPR)
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
				fmt.Fprintf(t.w(), "\t%s %s, %s\n", cmd, dst, src)
			}
		}

	case ir.Oload:
		fmt.Fprintf(t.w(), "\tldr %s, [%s]\n", t.formatRef(ins.To, ins.Cls, f), t.formatRef(ins.Arg[0], ir.Kl, f))
	case ir.Ostoreb:
		fmt.Fprintf(t.w(), "\tstrb %s, [%s]\n", t.formatRef(ins.Arg[0], ir.Kw, f), t.formatRef(ins.Arg[1], ir.Kl, f))
	case ir.Ostoreh:
		fmt.Fprintf(t.w(), "\tstrh %s, [%s]\n", t.formatRef(ins.Arg[0], ir.Kw, f), t.formatRef(ins.Arg[1], ir.Kl, f))
	case ir.Ostorew:
		fmt.Fprintf(t.w(), "\tstr %s, [%s]\n", t.formatRef(ins.Arg[0], ir.Kw, f), t.formatRef(ins.Arg[1], ir.Kl, f))
	case ir.Ostorel:
		fmt.Fprintf(t.w(), "\tstr %s, [%s]\n", t.formatRef(ins.Arg[0], ir.Kl, f), t.formatRef(ins.Arg[1], ir.Kl, f))
	case ir.Ostores, ir.Ostored:
		fmt.Fprintf(t.w(), "\tstr %s, [%s]\n", t.formatRef(ins.Arg[0], ins.Cls, f), t.formatRef(ins.Arg[1], ir.Kl, f))

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
		fmt.Fprintf(t.w(), "\t%s %s, %s\n", cmd, dst, src)

	case ir.Oswtof, ir.Osltof, ir.Ouwtof, ir.Oultof:
		dst := t.formatRef(ins.To, ins.Cls, f)
		srcCls := ir.Kw
		if ins.Op == ir.Osltof || ins.Op == ir.Oultof {
			srcCls = ir.Kl
		}
		src := t.formatRef(ins.Arg[0], srcCls, f)
		cmd := "scvtf"
		if ins.Op == ir.Ouwtof || ins.Op == ir.Oultof {
			cmd = "ucvtf"
		}
		fmt.Fprintf(t.w(), "\t%s %s, %s\n", cmd, dst, src)

	case ir.Oexts, ir.Otruncd:
		fmt.Fprintf(t.w(), "\tfcvt %s, %s\n", t.formatRef(ins.To, ins.Cls, f), t.formatRef(ins.Arg[0], ir.Kx, f))
	case ir.Ocast:
		fmt.Fprintf(t.w(), "\tfmov %s, %s\n", t.formatRef(ins.To, ins.Cls, f), t.formatRef(ins.Arg[0], ir.Kx, f))

	case ir.Ocall:
		fmt.Fprintf(t.w(), "\tbl %s\n", t.formatRef(ins.Arg[0], ir.Kl, f))

	case ir.Oalloc4, ir.Oalloc8, ir.Oalloc16:
		offset := (ins.To.Val + 1) * 8
		fmt.Fprintf(t.w(), "\tadd %s, x29, #%d\n", t.formatRef(ins.To, ir.Kl, f), offset)

	case ir.Oneg:
		dst := t.formatRef(ins.To, ins.Cls, f)
		src := t.formatRef(ins.Arg[0], ins.Cls, f)
		if ins.Cls.IsFloat() {
			fmt.Fprintf(t.w(), "\tfneg %s, %s\n", dst, src)
		} else {
			fmt.Fprintf(t.w(), "\tneg %s, %s\n", dst, src)
		}

	case ir.Oloadsb:
		addr := t.formatRef(ins.Arg[0], ir.Kl, f)
		dst := t.formatRef(ins.To, ins.Cls, f)
		fmt.Fprintf(t.w(), "\tldrsb %s, [%s]\n", dst, addr)

	case ir.Oloadub:
		addr := t.formatRef(ins.Arg[0], ir.Kl, f)
		wdst := t.formatRef(ins.To, ir.Kw, f)
		fmt.Fprintf(t.w(), "\tldrb %s, [%s]\n", wdst, addr)

	case ir.Oloadsh:
		addr := t.formatRef(ins.Arg[0], ir.Kl, f)
		dst := t.formatRef(ins.To, ins.Cls, f)
		fmt.Fprintf(t.w(), "\tldrsh %s, [%s]\n", dst, addr)

	case ir.Oloaduh:
		addr := t.formatRef(ins.Arg[0], ir.Kl, f)
		wdst := t.formatRef(ins.To, ir.Kw, f)
		fmt.Fprintf(t.w(), "\tldrh %s, [%s]\n", wdst, addr)

	case ir.Oloadsw:
		addr := t.formatRef(ins.Arg[0], ir.Kl, f)
		xdst := t.formatRef(ins.To, ir.Kl, f)
		fmt.Fprintf(t.w(), "\tldrsw %s, [%s]\n", xdst, addr)

	case ir.Oloaduw:
		addr := t.formatRef(ins.Arg[0], ir.Kl, f)
		wdst := t.formatRef(ins.To, ir.Kw, f)
		fmt.Fprintf(t.w(), "\tldr %s, [%s]\n", wdst, addr)

	case ir.Oextsb:
		dst := t.formatRef(ins.To, ins.Cls, f)
		src := t.formatRef(ins.Arg[0], ir.Kw, f)
		fmt.Fprintf(t.w(), "\tsxtb %s, %s\n", dst, src)

	case ir.Oextub:
		wdst := t.formatRef(ins.To, ir.Kw, f)
		src := t.formatRef(ins.Arg[0], ir.Kw, f)
		fmt.Fprintf(t.w(), "\tuxtb %s, %s\n", wdst, src)

	case ir.Oextsh:
		dst := t.formatRef(ins.To, ins.Cls, f)
		src := t.formatRef(ins.Arg[0], ir.Kw, f)
		fmt.Fprintf(t.w(), "\tsxth %s, %s\n", dst, src)

	case ir.Oextuh:
		wdst := t.formatRef(ins.To, ir.Kw, f)
		src := t.formatRef(ins.Arg[0], ir.Kw, f)
		fmt.Fprintf(t.w(), "\tuxth %s, %s\n", wdst, src)

	case ir.Oextsw:
		xdst := t.formatRef(ins.To, ir.Kl, f)
		wsrc := t.formatRef(ins.Arg[0], ir.Kw, f)
		fmt.Fprintf(t.w(), "\tsxtw %s, %s\n", xdst, wsrc)

	case ir.Oextuw:
		wdst := t.formatRef(ins.To, ir.Kw, f)
		wsrc := t.formatRef(ins.Arg[0], ir.Kw, f)
		fmt.Fprintf(t.w(), "\tmov %s, %s\n", wdst, wsrc)

	case ir.Ocugew, ir.Ocugel:
		t.emitCmp("hs", ins, f)
	case ir.Ocugtw, ir.Ocugtl:
		t.emitCmp("hi", ins, f)
	case ir.Oculew, ir.Oculel:
		t.emitCmp("ls", ins, f)
	case ir.Ocultw, ir.Ocultl:
		t.emitCmp("lo", ins, f)

	case ir.Ocos, ir.Ocod:
		t.emitCmp("vc", ins, f)
	case ir.Ocuos, ir.Ocuod:
		t.emitCmp("vs", ins, f)

	case ir.Omadd:
		dst := t.formatRef(ins.To, ins.Cls, f)
		src1 := t.formatRef(ins.Arg[0], ins.Cls, f)
		src2 := t.formatRef(ins.Arg[1], ins.Cls, f)
		fmt.Fprintf(t.w(), "\tmadd %s, %s, %s, xzr\n", dst, src1, src2)

	default:
		fmt.Fprintf(t.w(), "\t// unhandled op %s\n", ins.Op)
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
	pfx := t.symPrefix()
	fmt.Fprintf(t.w(), ".data\n")
	if d.Exported {
		fmt.Fprintf(t.w(), ".globl %s%s\n", pfx, d.Label)
	}
	fmt.Fprintf(t.w(), ".p2align 3\n")
	fmt.Fprintf(t.w(), "%s%s:\n", pfx, d.Label)

	for _, item := range d.Items {
		switch item.Type {
		case "b":
			if item.String != "" {
				// We need to properly quote the string for assembly
				// But Go's %q adds quotes. Assembly expects quoted string too.
				// Example: .asciz "Hello\n"
				fmt.Fprintf(t.w(), "\t.asciz %q\n", item.String)
			} else {
				fmt.Fprintf(t.w(), "\t.byte %d\n", item.Value)
			}
		case "h":
			fmt.Fprintf(t.w(), "\t.hword %d\n", item.Value)
		case "w":
			fmt.Fprintf(t.w(), "\t.word %d\n", item.Value)
		case "l":
			if item.Label != "" {
				// If label refers to a symbol/global
				fmt.Fprintf(t.w(), "\t.quad %s%s\n", pfx, item.Label)
			} else {
				fmt.Fprintf(t.w(), "\t.quad %d\n", item.Value)
			}
		case "z":
			fmt.Fprintf(t.w(), "\t.zero %d\n", item.Value)
		}
	}
}
