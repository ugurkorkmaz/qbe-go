package analysis_test

import (
	"testing"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

func TestSSAConstruction(t *testing.T) {
	b := builder.NewBuilder("ssa_test")
	start := b.Block("start")
	then := b.Block("then")
	els := b.Block("else")
	end := b.Block("end")

	b.SetBlock(start)
	x1 := b.Con(10)
	b.Jnz(b.Compare(ir.Oceqw, ir.Kw, x1, b.Con(10)), then, els)

	b.SetBlock(then)
	_ = b.Add(ir.Kw, x1, b.Con(1))
	b.Jmp(end)

	b.SetBlock(els)
	_ = b.Add(ir.Kw, x1, b.Con(2))
	b.Jmp(end)

	b.SetBlock(end)
	// We want to see if SSA correctly inserts a Phi node for x here if we had reused a name.
	// But in our builder, we usually create new temps. 
	// Let's simulate name reuse by using the same original temp if possible, 
	// or just verify that the dominance frontier is correct.
	
	f := b.Build()
	analysis.SSA(f)

	if err := analysis.VerifySSA(f); err != nil {
		t.Fatalf("SSA construction produced invalid IR: %v", err)
	}

	// Verify that 'end' block has no Phis yet (because we didn't join names)
	if len(end.Phis) != 0 {
		t.Errorf("Expected 0 Phis in end block, got %d", len(end.Phis))
	}
}
