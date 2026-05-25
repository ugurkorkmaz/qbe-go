package arm64

import (
	"fmt"

	"github.com/ugurkorkmaz/qbe-go/ir"
)

func (t *ARM64Target) symPrefix() string {
	if t.Apple { return "_" }
	return ""
}

func (t *ARM64Target) Emit(f *ir.Function, globals []string) error {
	pfx := t.symPrefix()
	fmt.Fprintf(t.w(), ".text\n")
	if f.Exported { fmt.Fprintf(t.w(), ".globl %s%s\n", pfx, f.Name) }
	fmt.Fprintf(t.w(), ".p2align 2\n")
	fmt.Fprintf(t.w(), "%s%s:\n", pfx, f.Name)

	calleeSaveArea := t.calleeSaveAreaSize(f)
	prologueSize := 16 + calleeSaveArea
	fmt.Fprintf(t.w(), "\tstp x29, x30, [sp, #-%d]!\n", prologueSize)
	for i, reg := range f.CalleeSaved {
		fmt.Fprintf(t.w(), "\tstr %s, [sp, #%d]\n", gprNames[reg], 16+i*8)
	}
	fmt.Fprintf(t.w(), "\tmov x29, sp\n")

	frameSize := t.calculateFrame(f)
	if frameSize > 0 { t.emitFrameAlloc(frameSize) }

	t.CurrentGlobals = globals
	for _, b := range f.Blocks {
		lblPrefix := ".L"
		if t.Apple { lblPrefix = "L" }
		fmt.Fprintf(t.w(), "%s%s_%d:\t// @%s\n", lblPrefix, f.Name, b.Id, b.Name)
		
		t.PendingCmp = nil
		for _, ins := range b.Ins {
			t.emitIns(ins, f)
		}
		t.emitJump(b, f)
	}
	return nil
}

func (t *ARM64Target) calleeSaveAreaSize(f *ir.Function) int {
	n := len(f.CalleeSaved) * 8
	return (n + 15) &^ 15
}

func (t *ARM64Target) calculateFrame(f *ir.Function) int {
	maxSlot := -1
	for _, tmp := range f.Temps {
		if tmp.Slot > maxSlot { maxSlot = tmp.Slot }
	}
	if maxSlot == -1 { return 0 }
	return (maxSlot + 1) * 8
}

func (t *ARM64Target) emitFrameAlloc(size int) {
	size = (size + 15) &^ 15
	if size <= 4095 {
		fmt.Fprintf(t.w(), "\tsub sp, sp, #%d\n", size)
	} else {
		fmt.Fprintf(t.w(), "\tmov x16, #%d\n", size&0xffff)
		if size > 0xffff { fmt.Fprintf(t.w(), "\tmovk x16, #%d, lsl #16\n", (size>>16)&0xffff) }
		fmt.Fprintf(t.w(), "\tsub sp, sp, x16\n")
	}
}

func (t *ARM64Target) emitJump(b *ir.Block, f *ir.Function) {
	lblPrefix := ".L"
	if t.Apple { lblPrefix = "L" }

	if b.Jmp.Type == ir.Jjnz && t.PendingCmp != nil && t.PendingCmp.To == b.Jmp.Arg {
		// Branch fusion: cmp + b.cond
		t.emitCmpOnly(*t.PendingCmp, f)
		cond := t.cmpCond(t.PendingCmp.Op)
		fmt.Fprintf(t.w(), "\tb.%s %s%s_%d\n", cond, lblPrefix, f.Name, b.S1.Id)
		fmt.Fprintf(t.w(), "\tb %s%s_%d\n", lblPrefix, f.Name, b.S2.Id)
		t.PendingCmp = nil
		return
	}

	t.flushPendingCmp(f)

	switch b.Jmp.Type {
	case ir.Jretw, ir.Jretl, ir.Jrets, ir.Jretd:
		cls := b.Jmp.Type.Class()
		dst := ir.PhysicalReg(0)
		if cls.IsFloat() { dst = ir.PhysicalReg(32) }
		if !b.Jmp.Arg.IsUndef() {
			val := t.formatRef(b.Jmp.Arg, cls, f)
			dstReg := t.formatRef(dst, cls, f)
			if dstReg != val { fmt.Fprintf(t.w(), "\tmov %s, %s\n", dstReg, val) }
		}
		t.emitEpilogue(f)
	case ir.Jjmp:
		if b.S1 != nil { fmt.Fprintf(t.w(), "\tb %s%s_%d\n", lblPrefix, f.Name, b.S1.Id)
		} else { t.emitEpilogue(f) }
	case ir.Jjnz:
		fmt.Fprintf(t.w(), "\tcbnz %s, %s%s_%d\n", t.formatRef(b.Jmp.Arg, ir.Kw, f), lblPrefix, f.Name, b.S1.Id)
		fmt.Fprintf(t.w(), "\tb %s%s_%d\n", lblPrefix, f.Name, b.S2.Id)
	}
}

func (t *ARM64Target) emitEpilogue(f *ir.Function) {
	calleeSaveArea := t.calleeSaveAreaSize(f)
	prologueSize := 16 + calleeSaveArea
	fmt.Fprintf(t.w(), "\tmov sp, x29\n")
	for i, reg := range f.CalleeSaved {
		fmt.Fprintf(t.w(), "\tldr %s, [sp, #%d]\n", gprNames[reg], 16+i*8)
	}
	fmt.Fprintf(t.w(), "\tldp x29, x30, [sp], #%d\n", prologueSize)
	fmt.Fprintf(t.w(), "\tret\n")
}

func (t *ARM64Target) flushPendingCmp(f *ir.Function) {
	if t.PendingCmp != nil {
		ins := *t.PendingCmp
		t.PendingCmp = nil
		t.emitInsDirect(ins, f)
	}
}

func (t *ARM64Target) emitIns(ins ir.Instruction, f *ir.Function) {
	// If instruction is a comparison, delay it for potential fusion
	switch ins.Op {
	case ir.Oceqw, ir.Oceql, ir.Ocnew, ir.Ocnel, ir.Ocsltw, ir.Ocsltl, ir.Ocslew, ir.Ocslel, ir.Ocsgtw, ir.Ocsgtl, ir.Ocsgew, ir.Ocsgel:
		t.flushPendingCmp(f)
		t.PendingCmp = &ins
		return
	}

	// If this instruction uses the pending cmp result, flush it
	if t.PendingCmp != nil {
		if ins.Arg[0] == t.PendingCmp.To || ins.Arg[1] == t.PendingCmp.To {
			t.flushPendingCmp(f)
		}
	}

	t.emitInsDirect(ins, f)
}

func (t *ARM64Target) emitInsDirect(ins ir.Instruction, f *ir.Function) {
	switch ins.Op {
	case ir.Onop:
		return
	case ir.Oadd: t.emitBinop("add", ins, f)
	case ir.Osub: t.emitBinop("sub", ins, f)
	case ir.Omul: t.emitBinop("mul", ins, f)
	case ir.Odiv, ir.Oudiv:
		op := "sdiv"
		if ins.Op == ir.Oudiv { op = "udiv" }
		t.emitBinop(op, ins, f)
	case ir.Ocopy:
		dst := t.formatRef(ins.To, ins.Cls, f)
		src := ins.Arg[0]
		if src.Kind == ir.RSym {
			sym := t.formatRef(src, ins.Cls, f)
			fmt.Fprintf(t.w(), "\tadrp %s, %s\n", dst, sym)
			fmt.Fprintf(t.w(), "\tadd %s, %s, :lo12:%s\n", dst, dst, sym)
		} else if src.Kind == ir.RCon || src.Kind == ir.RInt {
			val := uint64(src.Val)
			if src.Kind == ir.RCon { val = f.Constants[src.Val].Val }
			t.emitMoveImm(dst, val, ins.Cls == ir.Kl)
		} else {
			srcReg := t.formatRef(src, ins.Cls, f)
			if dst != srcReg {
				cmd := "mov"
				if ins.Cls.IsFloat() { cmd = "fmov" }
				fmt.Fprintf(t.w(), "\t%s %s, %s\n", cmd, dst, srcReg)
			}
		}
	case ir.Oload: t.emitMem("ldr", t.formatRef(ins.To, ins.Cls, f), ins.Arg[0], f)
	case ir.Oloadub: t.emitMem("ldrb", t.formatRef(ins.To, ir.Kw, f), ins.Arg[0], f)
	case ir.Ostoreb: t.emitMem("strb", t.formatRef(ins.Arg[0], ir.Kw, f), ins.Arg[1], f)
	case ir.Ostorel: t.emitMem("str", t.formatRef(ins.Arg[0], ir.Kl, f), ins.Arg[1], f)
	case ir.Omadd, ir.Omsub:
		dst := t.formatRef(ins.To, ins.Cls, f)
		src1 := t.formatRef(ins.Arg[0], ins.Cls, f)
		src2 := t.formatRef(ins.Arg[1], ins.Cls, f)
		src3 := t.formatRef(ins.Arg[2], ins.Cls, f)
		cmd := "madd"
		if ins.Op == ir.Omsub { cmd = "msub" }
		fmt.Fprintf(t.w(), "\t%s %s, %s, %s, %s\n", cmd, dst, src1, src2, src3)
	case ir.Ocall:
		t.flushPendingCmp(f)
		fmt.Fprintf(t.w(), "\tbl %s\n", t.formatRef(ins.Arg[0], ir.Kl, f))
	case ir.Oalloc4, ir.Oalloc8, ir.Oalloc16:
		off := int((ins.To.Val + 1) * 8)
		t.emitFrameAllocForReg(t.formatRef(ins.To, ir.Kl, f), off)
	case ir.Oceqw, ir.Oceql, ir.Oceqs, ir.Oceqd: t.emitCmp("eq", ins, f)
	case ir.Ocnew, ir.Ocnel, ir.Ocnes, ir.Ocned: t.emitCmp("ne", ins, f)
	case ir.Ocsltw, ir.Ocsltl, ir.Oclts, ir.Ocltd: t.emitCmp("lt", ins, f)
	case ir.Ocslew, ir.Ocslel, ir.Ocles, ir.Ocled: t.emitCmp("le", ins, f)
	case ir.Ocsgtw, ir.Ocsgtl, ir.Ocgts, ir.Ocgtd: t.emitCmp("gt", ins, f)
	case ir.Ocsgew, ir.Ocsgel, ir.Ocges, ir.Ocged: t.emitCmp("ge", ins, f)
	case ir.Oneg:
		dst := t.formatRef(ins.To, ins.Cls, f)
		fmt.Fprintf(t.w(), "\tneg %s, %s\n", dst, t.formatRef(ins.Arg[0], ins.Cls, f))
	default: fmt.Fprintf(t.w(), "\t// unhandled op %s\n", ins.Op)
	}
}

func (t *ARM64Target) emitCmpOnly(ins ir.Instruction, f *ir.Function) {
	src1 := t.formatRef(ins.Arg[0], ins.Cls, f)
	src2 := t.formatRef(ins.Arg[1], ins.Cls, f)
	if ins.Cls.IsFloat() {
		fmt.Fprintf(t.w(), "\tfcmp %s, %s\n", src1, src2)
	} else {
		fmt.Fprintf(t.w(), "\tcmp %s, %s\n", src1, src2)
	}
}

func (t *ARM64Target) cmpCond(op ir.Opcode) string {
	switch op {
	case ir.Oceqw, ir.Oceql, ir.Oceqs, ir.Oceqd: return "eq"
	case ir.Ocnew, ir.Ocnel, ir.Ocnes, ir.Ocned: return "ne"
	case ir.Ocsltw, ir.Ocsltl, ir.Oclts, ir.Ocltd: return "lt"
	case ir.Ocslew, ir.Ocslel, ir.Ocles, ir.Ocled: return "le"
	case ir.Ocsgtw, ir.Ocsgtl, ir.Ocgts, ir.Ocgtd: return "gt"
	case ir.Ocsgew, ir.Ocsgel, ir.Ocges, ir.Ocged: return "ge"
	default: return "al"
	}
}

func (t *ARM64Target) emitFrameAllocForReg(reg string, size int) {
	if size <= 4095 {
		fmt.Fprintf(t.w(), "\tadd %s, x29, #%d\n", reg, size)
	} else {
		fmt.Fprintf(t.w(), "\tmov x16, #%d\n", size&0xffff)
		if size > 0xffff { fmt.Fprintf(t.w(), "\tmovk x16, #%d, lsl #16\n", (size>>16)&0xffff) }
		fmt.Fprintf(t.w(), "\tadd %s, x29, x16\n", reg)
	}
}

func (t *ARM64Target) emitMem(cmd string, r string, addr ir.Ref, f *ir.Function) {
	if addr.Kind == ir.RSlot {
		off := int((addr.Val + 1) * 8)
		if off <= 4095 {
			fmt.Fprintf(t.w(), "\t%s %s, [x29, #%d]\n", cmd, r, off)
		} else {
			fmt.Fprintf(t.w(), "\tmov x16, #%d\n", off&0xffff)
			if off > 0xffff { fmt.Fprintf(t.w(), "\tmovk x16, #%d, lsl #16\n", (off>>16)&0xffff) }
			fmt.Fprintf(t.w(), "\tadd x16, x29, x16\n")
			fmt.Fprintf(t.w(), "\t%s %s, [x16]\n", cmd, r)
		}
	} else {
		fmt.Fprintf(t.w(), "\t%s %s, [%s]\n", cmd, r, t.formatRef(addr, ir.Kl, f))
	}
}

func (t *ARM64Target) formatRef(r ir.Ref, cls ir.Class, f *ir.Function) string {
	switch r.Kind {
	case ir.RReg:
		if r.Val == 31 { return "sp" }
		if r.Val >= 32 && r.Val < 64 {
			if cls == ir.Ks { return fmt.Sprintf("s%d", r.Val-32) }
			return fmt.Sprintf("d%d", r.Val-32)
		}
		if cls == ir.Kw { return gpr32Names[r.Val] }
		return gprNames[r.Val]
	case ir.RCon: return fmt.Sprintf("#%d", f.Constants[r.Val].Val)
	case ir.RInt: return fmt.Sprintf("#%d", int32(r.Val))
	case ir.RSlot: return fmt.Sprintf("[x29, #%d]", (r.Val+1)*8)
	case ir.RSym:
		idx := int(r.Val)
		if idx < len(t.CurrentGlobals) { return t.CurrentGlobals[idx] }
		return fmt.Sprintf("sym_%d", r.Val)
	default: return fmt.Sprintf("VREG_%d", r.Val)
	}
}

func (t *ARM64Target) EmitData(d *ir.Data) {
	pfx := t.symPrefix()
	fmt.Fprintf(t.w(), ".data\n")
	if d.Exported { fmt.Fprintf(t.w(), ".globl %s%s\n", pfx, d.Label) }
	fmt.Fprintf(t.w(), ".p2align 3\n")
	fmt.Fprintf(t.w(), "%s%s:\n", pfx, d.Label)
	for _, item := range d.Items {
		switch item.Type {
		case "b":
			if item.String != "" { fmt.Fprintf(t.w(), "\t.asciz %q\n", item.String)
			} else { fmt.Fprintf(t.w(), "\t.byte %d\n", item.Value) }
		case "h": fmt.Fprintf(t.w(), "\t.hword %d\n", item.Value)
		case "w": fmt.Fprintf(t.w(), "\t.word %d\n", item.Value)
		case "l":
			if item.Label != "" { fmt.Fprintf(t.w(), "\t.quad %s%s\n", pfx, item.Label)
			} else { fmt.Fprintf(t.w(), "\t.quad %d\n", item.Value) }
		case "z": fmt.Fprintf(t.w(), "\t.zero %d\n", item.Value)
		}
	}
}
