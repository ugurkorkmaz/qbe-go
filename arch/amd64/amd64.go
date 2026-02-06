package amd64

import (
	"fmt"

	"github.com/ugurkorkmaz/qbe-go/ir"
)

// AMD64 specific register constants according to System V ABI
const (
	RAX  = iota // 0
	RCX         // 1
	RDX         // 2
	RBX         // 3
	RSP         // 4
	RBP         // 5
	RSI         // 6
	RDI         // 7
	R8          // 8
	R9          // 9
	R10         // 10
	R11         // 11
	R12         // 12
	R13         // 13
	R14         // 14
	R15         // 15
	XMM0        // 16
	XMM1
	XMM2
	XMM3
	XMM4
	XMM5
	XMM6
	XMM7
)

var gprNames = []string{
	"rax", "rcx", "rdx", "rbx", "rsp", "rbp", "rsi", "rdi",
	"r8", "r9", "r10", "r11", "r12", "r13", "r14", "r15",
}

var gpr32Names = []string{
	"eax", "ecx", "edx", "ebx", "esp", "ebp", "esi", "edi",
	"r8d", "r9d", "r10d", "r11d", "r12d", "r13d", "r14d", "r15d",
}

// System V ABI parameter registers
var sysvGPR = []int{RDI, RSI, RDX, RCX, R8, R9}
var sysvFPR = []int{XMM0, XMM1, XMM2, XMM3, XMM4, XMM5, XMM6, XMM7}

type AMD64Target struct{}

func (t *AMD64Target) Name() string { return "amd64" }
func (t *AMD64Target) GPR0() int    { return RAX }
func (t *AMD64Target) NGPR() int    { return 16 }
func (t *AMD64Target) FPR0() int    { return XMM0 }
func (t *AMD64Target) NFPR() int    { return 8 }

func (t *AMD64Target) RegName(r int) string {
	if r >= 0 && r < 16 {
		return gprNames[r]
	}
	if r >= 16 && r < 24 {
		return fmt.Sprintf("xmm%d", r-16)
	}
	return fmt.Sprintf("reg%d", r)
}

func (t *AMD64Target) ABI0(f *ir.Function) {
	t.lowerParams(f)
	for _, b := range f.Blocks {
		t.lowerRet(f, b)
		t.lowerCalls(f, b)
	}
}

func (t *AMD64Target) lowerParams(f *ir.Function) {
	b := f.Start
	if b == nil {
		return
	}
	var newIns []ir.Instruction
	gprIdx, fprIdx := 0, 0
	for _, ins := range b.Ins {
		if ins.Op == ir.Opar {
			var reg int
			if ins.Cls.IsFloat() {
				if fprIdx < len(sysvFPR) {
					reg = sysvFPR[fprIdx]
					fprIdx++
				}
			} else {
				if gprIdx < len(sysvGPR) {
					reg = sysvGPR[gprIdx]
					gprIdx++
				}
			}
			newIns = append(newIns, ir.Instruction{
				Op:  ir.Ocopy,
				Cls: ins.Cls,
				To:  ins.To,
				Arg: [2]ir.Ref{ir.PhysicalReg(reg), ir.Undef},
			})
		} else {
			newIns = append(newIns, ins)
		}
	}
	b.Ins = newIns
}

func (t *AMD64Target) lowerRet(f *ir.Function, b *ir.Block) {
	if b.Jmp.Type == ir.Jretw || b.Jmp.Type == ir.Jretl {
		b.Ins = append(b.Ins, ir.Instruction{
			Op:  ir.Ocopy,
			Cls: ir.Kw,
			To:  ir.PhysicalReg(RAX),
			Arg: [2]ir.Ref{b.Jmp.Arg, ir.Undef},
		})
		b.Jmp.Type, b.Jmp.Arg = ir.Jjmp, ir.Undef
	}
}

func (t *AMD64Target) lowerCalls(f *ir.Function, b *ir.Block) {
	// ... (simplified)
}

func (t *AMD64Target) Emit(f *ir.Function) error {
	fmt.Printf(".text\n")
	fmt.Printf(".globl %s\n", f.Name)
	fmt.Printf("%s:\n", f.Name)

	// Prologue
	fmt.Printf("\tpushq %%rbp\n")
	fmt.Printf("\tmovq %%rsp, %%rbp\n")

	// Frame size calculation (stub)
	// We'd need to know how many slots were used
	frameSize := 0
	for _, tmp := range f.Temps {
		if tmp.Slot != -1 {
			frameSize += 8 // simplifed
		}
	}
	if frameSize > 0 {
		fmt.Printf("\tsubq $%d, %%rsp\n", (frameSize+15)&^15)
	}

	for _, b := range f.Blocks {
		if b.Name != "start" {
			fmt.Printf(".L%d:\t# @%s\n", b.Id, b.Name)
		}
		for _, ins := range b.Ins {
			t.emitIns(ins, f)
		}
		// Jump
		switch b.Jmp.Type {
		case ir.Jjmp:
			if b.S1 != nil {
				fmt.Printf("\tjmp .L%d\n", b.S1.Id)
			} else {
				// Epilogue for return
				fmt.Printf("\tleave\n")
				fmt.Printf("\tret\n")
			}
		case ir.Jjnz:
			fmt.Printf("\ttestl %s, %s\n", t.formatRef(b.Jmp.Arg, ir.Kw, f), t.formatRef(b.Jmp.Arg, ir.Kw, f))
			fmt.Printf("\tjnz .L%d\n", b.S1.Id)
			fmt.Printf("\tjmp .L%d\n", b.S2.Id)
		}
	}
	return nil
}

func (t *AMD64Target) emitIns(ins ir.Instruction, f *ir.Function) {
	switch ins.Op {
	case ir.Onop:
		return
	case ir.Oadd:
		fmt.Printf("\tadd%s %s, %s\n", t.asmCls(ins.Cls), t.formatRef(ins.Arg[1], ins.Cls, f), t.formatRef(ins.Arg[0], ins.Cls, f))
		if ins.To != ins.Arg[0] {
			fmt.Printf("\tmov%s %s, %s\n", t.asmCls(ins.Cls), t.formatRef(ins.Arg[0], ins.Cls, f), t.formatRef(ins.To, ins.Cls, f))
		}
	case ir.Ocopy:
		fmt.Printf("\tmov%s %s, %s\n", t.asmCls(ins.Cls), t.formatRef(ins.Arg[0], ins.Cls, f), t.formatRef(ins.To, ins.Cls, f))
	case ir.Oceqw:
		fmt.Printf("\tcmpl %s, %s\n", t.formatRef(ins.Arg[1], ir.Kw, f), t.formatRef(ins.Arg[0], ir.Kw, f))
		fmt.Printf("\tsete %s\n", t.formatRef(ins.To, ir.Kw, f)) // simplified
		fmt.Printf("\tmovzb%s %s, %s\n", t.asmCls(ir.Kw), t.formatRef(ins.To, ir.Kw, f), t.formatRef(ins.To, ir.Kw, f))
	default:
		fmt.Printf("\t# unhandled op %s\n", ins.Op)
	}
}

func (t *AMD64Target) asmCls(cls ir.Class) string {
	switch cls {
	case ir.Kw:
		return "l"
	case ir.Kl:
		return "q"
	case ir.Ks:
		return "ss"
	case ir.Kd:
		return "sd"
	default:
		return "l"
	}
}

func (t *AMD64Target) formatRef(r ir.Ref, cls ir.Class, f *ir.Function) string {
	switch r.Kind {
	case ir.RReg:
		if cls == ir.Kw {
			return "%" + gpr32Names[r.Val]
		}
		return "%" + gprNames[r.Val]
	case ir.RCon:
		return fmt.Sprintf("$%d", f.Constants[r.Val].Val)
	case ir.RInt:
		return fmt.Sprintf("$%d", int32(r.Val))
	case ir.RSlot:
		return fmt.Sprintf("-%d(%%rbp)", (r.Val+1)*8)
	default:
		return r.String()
	}
}
