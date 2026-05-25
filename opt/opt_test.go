package opt_test

import (
	"testing"

	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/ir"
	"github.com/ugurkorkmaz/qbe-go/opt"
)

func TestFold(t *testing.T) {
	tests := []struct {
		op       ir.Opcode
		cls      ir.Class
		l, r     uint64
		expected uint64
		ok       bool
	}{
		{ir.Oadd, ir.Kw, 10, 20, 30, true},
		{ir.Osub, ir.Kw, 50, 10, 40, true},
		{ir.Omul, ir.Kw, 6, 7, 42, true},
		{ir.Odiv, ir.Kw, 100, 10, 10, true},
		{ir.Oand, ir.Kw, 0xF0, 0x0F, 0, true},
		{ir.Oor, ir.Kw, 0xF0, 0x0F, 0xFF, true},
	}

	f := &ir.Function{}
	for _, tc := range tests {
		res, ok := opt.Fold(f, tc.op, tc.cls, ir.Constant{Val: tc.l}, ir.Constant{Val: tc.r})
		if ok != tc.ok {
			t.Errorf("Fold(%v, %v, %v) ok mismatch: expected %v, got %v", tc.op, tc.l, tc.r, tc.ok, ok)
		}
		if ok && res.Val != tc.expected {
			t.Errorf("Fold(%v, %v, %v) value mismatch: expected %v, got %v", tc.op, tc.l, tc.r, tc.expected, res.Val)
		}
	}
}

func TestDCE(t *testing.T) {
	b := builder.NewBuilder("dce")
	b.Block("start")
	
	// Dead variable
	_ = b.Add(ir.Kw, b.Con(1), b.Con(2))
	
	// Live variable
	v2 := b.Add(ir.Kw, b.Con(10), b.Con(20))
	b.Ret(ir.Kw, v2)
	
	f := b.Build()
	// Before DCE, we should have 2 adds
	if len(f.Blocks[0].Ins) != 2 {
		t.Errorf("Expected 2 instructions before DCE, got %d", len(f.Blocks[0].Ins))
	}
	
	opt.DCE(f)
	
	// After DCE, only 1 add should remain
	if len(f.Blocks[0].Ins) != 1 {
		t.Errorf("Expected 1 instruction after DCE, got %d", len(f.Blocks[0].Ins))
	}
}
