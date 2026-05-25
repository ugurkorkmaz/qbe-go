package codegen_test

import (
	"testing"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/arch/arm64"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/codegen"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

func TestRegAlloc(t *testing.T) {
	b := builder.NewBuilder("ra")
	b.Block("start")
	
	p1 := b.Param(ir.Kw, "a")
	v1 := b.Add(ir.Kw, p1, b.Con(10))
	v2 := b.Add(ir.Kw, v1, b.Con(20))
	b.Ret(ir.Kw, v2)
	
	f := b.Build()
	target := &arm64.ARM64Target{Apple: false}
	
	analysis.SSA(f)
	codegen.Spill(f, target)
	target.ABI0(f)
	
	ra := codegen.NewRegAllocator(f, target)
	ra.Allocate()
	
	// Check if all used temporaries are now physical registers
	for _, blk := range f.Blocks {
		for _, ins := range blk.Ins {
			if ins.To.IsTmp() {
				t.Errorf("Instruction result is still a temporary: %v", ins.To)
			}
			for _, arg := range ins.Arg {
				if arg.IsTmp() {
					t.Errorf("Instruction argument is still a temporary: %v", arg)
				}
			}
		}
		if blk.Jmp.Arg.IsTmp() {
			t.Errorf("Jump argument is still a temporary: %v", blk.Jmp.Arg)
		}
	}
}
