package arm64

import (
	"github.com/ugurkorkmaz/qbe-go/ir"
)

// Simplify performs ARM64-specific peephole optimizations.
func (t *ARM64Target) Simplify(f *ir.Function) {
	for _, b := range f.Blocks {
		for i := range b.Ins {
			ins := &b.Ins[i]

			// 1. madd fusion: dst = (a * b) + c
			if ins.Op == ir.Oadd && !ins.Cls.IsFloat() {
				for n := 0; n < 2; n++ {
					arg := ins.Arg[n]
					other := ins.Arg[1-n]
					
					if !arg.IsTmp() || !other.IsTmp() {
						continue
					}

					def := f.Temps[arg.Val].Def
					if def != nil && def.Op == ir.Omul && def.Cls == ins.Cls {
						a, b_op := def.Arg[0], def.Arg[1]
						
						if a.IsTmp() && b_op.IsTmp() && f.Temps[arg.Val].NUse == 1 {
							ins.Op = ir.Omadd
							ins.Arg[0] = a
							ins.Arg[1] = b_op
							ins.Arg[2] = other
							def.Op = ir.Onop
							break
						}
					}
				}
			}

			// 2. msub fusion: dst = c - (a * b)
			if ins.Op == ir.Osub && !ins.Cls.IsFloat() {
				arg0, arg1 := ins.Arg[0], ins.Arg[1]
				if arg0.IsTmp() && arg1.IsTmp() {
					def := f.Temps[arg1.Val].Def
					if def != nil && def.Op == ir.Omul && def.Cls == ins.Cls {
						a, b_op := def.Arg[0], def.Arg[1]
						if a.IsTmp() && b_op.IsTmp() && f.Temps[arg1.Val].NUse == 1 {
							ins.Op = ir.Omsub
							ins.Arg[0] = a
							ins.Arg[1] = b_op
							ins.Arg[2] = arg0 // c
							def.Op = ir.Onop
						}
					}
				}
			}
		}
	}
}
