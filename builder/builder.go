package builder

import (
	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

type Builder struct {
	Func *ir.Function
	cur  *ir.Block
}

func NewBuilder(name string) *Builder {
	f := &ir.Function{Name: name, NTmp: 64} // Reserve 0-63 for physical regs
	// Pre-allocate Temps for physicals
	f.Temps = make([]ir.Temporary, 64)
	for i := range f.Temps {
		f.Temps[i].Slot = -1
	}
	return &Builder{Func: f}
}

func (b *Builder) Block(name string) *ir.Block {
	blk := &ir.Block{Name: name, Id: b.Func.NBlk}
	b.Func.NBlk++
	b.Func.Blocks = append(b.Func.Blocks, blk)
	if b.Func.Start == nil {
		b.Func.Start = blk
	}
	b.cur = blk
	return blk
}

func (b *Builder) SetBlock(blk *ir.Block) { b.cur = blk }

func (b *Builder) Tmp(name string, cls ir.Class) ir.Ref {
	return b.Func.NewTmp(name, cls)
}

func (b *Builder) Con(val uint64) ir.Ref {
	return b.Func.GetCon(val)
}

func (b *Builder) Add(cls ir.Class, l, r ir.Ref) ir.Ref {
	to := b.Tmp("", cls)
	b.Ins(ir.Oadd, cls, to, l, r)
	return to
}

func (b *Builder) Copy(cls ir.Class, src ir.Ref) ir.Ref {
	to := b.Tmp("", cls)
	b.Ins(ir.Ocopy, cls, to, src, ir.Undef)
	return to
}
func (b *Builder) Compare(op ir.Opcode, cls ir.Class, l, r ir.Ref) ir.Ref {
	to := b.Tmp("", ir.Kw)
	b.Ins(op, cls, to, l, r)
	return to
}

func (b *Builder) Ins(op ir.Opcode, cls ir.Class, to, arg1, arg2 ir.Ref) {
	b.cur.Ins = append(b.cur.Ins, ir.Instruction{
		Op:  op,
		Cls: cls,
		To:  to,
		Arg: [2]ir.Ref{arg1, arg2},
	})
}

func (b *Builder) Jmp(target *ir.Block) {
	b.cur.Jmp = ir.Jump{Type: ir.Jjmp}
	b.cur.S1 = target
}

func (b *Builder) Jnz(arg ir.Ref, s1, s2 *ir.Block) {
	b.cur.Jmp = ir.Jump{Type: ir.Jjnz, Arg: arg}
	b.cur.S1 = s1
	b.cur.S2 = s2
}

func (b *Builder) Ret(cls ir.Class, arg ir.Ref) {
	var jtype ir.JumpType
	switch cls {
	case ir.Kw:
		jtype = ir.Jretw
	case ir.Kl:
		jtype = ir.Jretl
	default:
		jtype = ir.Jretw
	}
	b.cur.Jmp = ir.Jump{Type: jtype, Arg: arg}
}

func (b *Builder) Param(cls ir.Class, name string) ir.Ref {
	to := b.Tmp(name, cls)
	b.Ins(ir.Opar, cls, to, ir.Undef, ir.Undef)
	return to
}

func (b *Builder) Call(cls ir.Class, addr ir.Ref, args []ir.Ref) ir.Ref {
	for _, arg := range args {
		acls := ir.Kl // Default to 64-bit int
		if arg.Kind == ir.RTmp {
			acls = b.Func.Temps[arg.Val].Cls
		} else if arg.Kind == ir.RCon {
			val := b.Func.Constants[arg.Val].Val
			if val <= 0xFFFFFFFF {
				acls = ir.Kw
			}
		} else if arg.Kind == ir.RInt {
			// RInt is usually small
			acls = ir.Kw
		}
		b.Ins(ir.Oarg, acls, ir.Undef, arg, ir.Undef)
	}
	to := b.Tmp("", cls)
	b.Ins(ir.Ocall, cls, to, addr, ir.Undef)
	return to
}

func (b *Builder) Build() *ir.Function {
	analysis.FillRPO(b.Func)
	analysis.FillPreds(b.Func)
	return b.Func
}
