package arm64

import (
	"fmt"

	"github.com/ugurkorkmaz/qbe-go/ir"
)

func (t *ARM64Target) emitModulo(ins ir.Instruction, f *ir.Function) {
	dst := t.formatRef(ins.To, ins.Cls, f)
	src1 := t.formatRef(ins.Arg[0], ins.Cls, f)
	src2 := t.formatRef(ins.Arg[1], ins.Cls, f)
	divOp := "sdiv"
	if ins.Op == ir.Ourem {
		divOp = "udiv"
	}

	tmp := "x16"
	if ins.Cls == ir.Kw {
		tmp = "w16"
	}
	fmt.Fprintf(t.w(), "\t%s %s, %s, %s\n", divOp, tmp, src1, src2)
	fmt.Fprintf(t.w(), "\tmsub %s, %s, %s, %s\n", dst, tmp, src2, src1)
}

func (t *ARM64Target) emitMoveImm(dst string, val uint64, wide bool) {
	if val == 0 {
		zr := "xzr"
		if !wide {
			zr = "wzr"
		}
		fmt.Fprintf(t.w(), "\tmov %s, %s\n", dst, zr)
		return
	}
	fmt.Fprintf(t.w(), "\tmovz %s, #%d\n", dst, val&0xffff)
	for shift := 16; shift < 64; shift += 16 {
		chunk := (val >> shift) & 0xffff
		if chunk != 0 {
			fmt.Fprintf(t.w(), "\tmovk %s, #%d, lsl #%d\n", dst, chunk, shift)
		}
		if !wide && shift == 16 {
			break
		}
	}
}

func (t *ARM64Target) emitBinop(name string, ins ir.Instruction, f *ir.Function) {
	if ins.Cls.IsFloat() {
		name = "f" + name
	}
	dst := t.formatRef(ins.To, ins.Cls, f)
	
	// ARM64 multiplication/division only take registers
	isRegOnly := false
	switch ins.Op {
	case ir.Omul, ir.Odiv, ir.Oudiv:
		isRegOnly = true
	}

	src1 := t.formatRef(ins.Arg[0], ins.Cls, f)
	if ins.Arg[0].Kind == ir.RCon || ins.Arg[0].Kind == ir.RInt {
		tmp1 := "x16"
		if ins.Cls == ir.Kw { tmp1 = "w16" }
		val := uint64(ins.Arg[0].Val)
		if ins.Arg[0].Kind == ir.RCon { val = f.Constants[ins.Arg[0].Val].Val }
		t.emitMoveImm(tmp1, val, ins.Cls == ir.Kl)
		src1 = tmp1
	}

	src2 := t.formatRef(ins.Arg[1], ins.Cls, f)
	if isRegOnly || ins.Arg[1].Kind == ir.RCon || ins.Arg[1].Kind == ir.RInt {
		if ins.Arg[1].Kind == ir.RCon || ins.Arg[1].Kind == ir.RInt {
			tmp2 := "x17"
			if ins.Cls == ir.Kw { tmp2 = "w17" }
			val := uint64(ins.Arg[1].Val)
			if ins.Arg[1].Kind == ir.RCon { val = f.Constants[ins.Arg[1].Val].Val }
			t.emitMoveImm(tmp2, val, ins.Cls == ir.Kl)
			src2 = tmp2
		}
	}

	if ins.Cls.IsFloat() {
		// Float ops usually need fmov first if src1/src2 are from GPRs
		// (Simplified for now)
		fmt.Fprintf(t.w(), "\t%s %s, %s, %s\n", name, dst, src1, src2)
	} else {
		fmt.Fprintf(t.w(), "\t%s %s, %s, %s\n", name, dst, src1, src2)
	}
}

func (t *ARM64Target) emitCmp(cond string, ins ir.Instruction, f *ir.Function) {
	src1 := t.formatRef(ins.Arg[0], ins.Cls, f)
	src2 := t.formatRef(ins.Arg[1], ins.Cls, f)

	if ins.Cls.IsFloat() {
		fmt.Fprintf(t.w(), "\tfcmp %s, %s\n", src1, src2)
	} else {
		fmt.Fprintf(t.w(), "\tcmp %s, %s\n", src1, src2)
	}
	fmt.Fprintf(t.w(), "\tcset %s, %s\n", t.formatRef(ins.To, ir.Kw, f), cond)
}
