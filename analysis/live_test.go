package analysis_test

import (
	"testing"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

func TestLiveness(t *testing.T) {
	b := builder.NewBuilder("liveness")
	start := b.Block("start")
	end := b.Block("end")

	b.SetBlock(start)
	v1 := b.Add(ir.Kw, b.Con(1), b.Con(2))
	b.Jmp(end)

	b.SetBlock(end)
	v2 := b.Add(ir.Kw, v1, b.Con(3))
	b.Ret(ir.Kw, v2)
	
	f := b.Build()
	// analysis.SSA(f) // SKIP SSA
	analysis.Liveness(f)
	
	startBlk := f.Blocks[0]
	endBlk := f.Blocks[1]
	
	if !startBlk.Out.Has(v1.Val) {
		t.Errorf("v1 (%d) should be live-out of start block", v1.Val)
	}
	if !endBlk.Gen.Has(v1.Val) {
		t.Errorf("v1 (%d) should be in end block's Gen set", v1.Val)
	}
}
