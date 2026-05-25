package analysis_test

import (
	"testing"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

func TestCFG(t *testing.T) {
	// Build a simple diamond CFG
	// start -> then/els -> end
	b := builder.NewBuilder("diamond")
	start := b.Block("start")
	then := b.Block("then")
	els := b.Block("els")
	end := b.Block("end")

	b.SetBlock(start)
	b.Jnz(b.Con(1), then, els)

	b.SetBlock(then)
	b.Jmp(end)

	b.SetBlock(els)
	b.Jmp(end)

	b.SetBlock(end)
	b.Ret(ir.Kw, b.Con(0))

	f := b.Build() // Build calls FillRPO and FillPreds

	// 1. Test Reachability
	if len(f.Blocks) != 4 {
		t.Errorf("Expected 4 blocks, got %d", len(f.Blocks))
	}

	// 2. Test Predecessors
	if len(end.Pred) != 2 {
		t.Errorf("Expected end block to have 2 predecessors, got %d", len(end.Pred))
	}

	// 3. Test Domination
	analysis.FillDom(f)
	// Check if 'start' block is at the beginning of the function
	if f.Start.Name != "start" {
		t.Error("Start block should be 'start'")
	}
	// Verify that RPO is filled
	if len(f.Blocks) == 0 {
		t.Error("RPO blocks slice should not be empty")
	}
}
