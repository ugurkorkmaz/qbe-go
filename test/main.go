package main

import (
	"fmt"
	"math"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/arch/arm64"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/codegen"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

func compile(target *arm64.ARM64Target, globals []string, f *ir.Function) {
	analysis.SSA(f)
	codegen.Spill(f, target)
	target.ABI0(f)
	codegen.NewRegAllocator(f, target).Allocate()
	if err := target.Emit(f, globals); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func main() {
	target := &arm64.ARM64Target{Apple: false}

	globals := []string{
		"add",         // 0
		"print_int",   // 1
		"print_str",   // 2
		"msg",         // 3
		"print_float", // 4
		"sub",         // 5
		"mul",         // 6
		"neg_w",       // 7
		"max_w",       // 8
		"is_positive", // 9
	}

	// --- Function: add(w %a, w %b) ---
	b1 := builder.NewBuilder("add")
	b1.Block("start")
	p1 := b1.Param(ir.Kw, "a")
	p2 := b1.Param(ir.Kw, "b")
	b1.Ret(ir.Kw, b1.Add(ir.Kw, p1, p2))

	// --- Test 1: sub(w %a, w %b) ---
	bSub := builder.NewBuilder("sub")
	bSub.Block("start")
	sa := bSub.Param(ir.Kw, "a")
	sb := bSub.Param(ir.Kw, "b")
	bSub.Ret(ir.Kw, bSub.Sub(ir.Kw, sa, sb))

	// --- Test 2: mul(w %a, w %b) ---
	bMul := builder.NewBuilder("mul")
	bMul.Block("start")
	ma := bMul.Param(ir.Kw, "a")
	mb := bMul.Param(ir.Kw, "b")
	bMul.Ret(ir.Kw, bMul.Mul(ir.Kw, ma, mb))

	// --- Test 3: neg_w(w %a) ---
	bNeg := builder.NewBuilder("neg_w")
	bNeg.Block("start")
	na := bNeg.Param(ir.Kw, "a")
	bNeg.Ret(ir.Kw, bNeg.Neg(ir.Kw, na))

	// --- Test 4: max_w(w %a, w %b) — uses branching ---
	bMax := builder.NewBuilder("max_w")
	startBlk := bMax.Block("start")
	thenBlk := bMax.Block("then")
	elseBlk := bMax.Block("else")

	bMax.SetBlock(startBlk)
	mxa := bMax.Param(ir.Kw, "a")
	mxb := bMax.Param(ir.Kw, "b")
	cmp := bMax.Compare(ir.Ocsgtw, ir.Kw, mxa, mxb)
	bMax.Jnz(cmp, thenBlk, elseBlk)

	bMax.SetBlock(thenBlk)
	bMax.Ret(ir.Kw, mxa)

	bMax.SetBlock(elseBlk)
	bMax.Ret(ir.Kw, mxb)

	// --- Test 5: is_positive(w %a) — returns 1 if a > 0 ---
	bPos := builder.NewBuilder("is_positive")
	bPos.Block("start")
	pa := bPos.Param(ir.Kw, "a")
	result := bPos.Compare(ir.Ocsgtw, ir.Kw, pa, ir.NewInt(0))
	bPos.Ret(ir.Kw, result)

	// --- Function: main() ---
	b2 := builder.NewBuilder("main")
	b2.Block("start")

	printIntSym := ir.Ref{Kind: ir.RSym, Val: 1}
	printStrSym := ir.Ref{Kind: ir.RSym, Val: 2}
	msgSym := ir.Ref{Kind: ir.RSym, Val: 3}
	printFloatSym := ir.Ref{Kind: ir.RSym, Val: 4}
	subSym := ir.Ref{Kind: ir.RSym, Val: 5}
	mulSym := ir.Ref{Kind: ir.RSym, Val: 6}
	negSym := ir.Ref{Kind: ir.RSym, Val: 7}
	maxSym := ir.Ref{Kind: ir.RSym, Val: 8}
	isPosSym := ir.Ref{Kind: ir.RSym, Val: 9}

	msgAddr := b2.Copy(ir.Kl, msgSym)
	b2.Call(ir.Kw, printStrSym, []ir.Ref{msgAddr})

	// add(10, 20) → 30
	r1 := b2.Call(ir.Kw, ir.Ref{Kind: ir.RSym, Val: 0}, []ir.Ref{b2.Con(10), b2.Con(20)})
	b2.Call(ir.Kw, printIntSym, []ir.Ref{r1})

	// print_float(3.14159)
	fval := b2.Con(math.Float64bits(3.14159))
	b2.Ins(ir.Oarg, ir.Kd, ir.Undef, fval, ir.Undef)
	b2.Ins(ir.Ocall, ir.Kw, ir.Undef, printFloatSym, ir.Undef)

	// sub(15, 6) → 9
	r2 := b2.Call(ir.Kw, subSym, []ir.Ref{b2.Con(15), b2.Con(6)})
	b2.Call(ir.Kw, printIntSym, []ir.Ref{r2})

	// mul(6, 7) → 42
	r3 := b2.Call(ir.Kw, mulSym, []ir.Ref{b2.Con(6), b2.Con(7)})
	b2.Call(ir.Kw, printIntSym, []ir.Ref{r3})

	// neg_w(5) → -5
	r4 := b2.Call(ir.Kw, negSym, []ir.Ref{b2.Con(5)})
	b2.Call(ir.Kw, printIntSym, []ir.Ref{r4})

	// max_w(3, 7) → 7
	r5 := b2.Call(ir.Kw, maxSym, []ir.Ref{b2.Con(3), b2.Con(7)})
	b2.Call(ir.Kw, printIntSym, []ir.Ref{r5})

	// is_positive(-1) → 0
	r6 := b2.Call(ir.Kw, isPosSym, []ir.Ref{b2.Con(^uint64(0))}) // -1 as uint64
	b2.Call(ir.Kw, printIntSym, []ir.Ref{r6})

	// is_positive(5) → 1
	r7 := b2.Call(ir.Kw, isPosSym, []ir.Ref{b2.Con(5)})
	b2.Call(ir.Kw, printIntSym, []ir.Ref{r7})

	b2.Ret(ir.Kw, b2.Con(0))

	f2 := b2.Build()
	f2.Exported = true

	msgData := &ir.Data{
		Label:    "msg",
		Exported: false,
		Items:    []ir.DataItem{{Type: "b", String: "Hello world from qbe-go!\n"}},
	}

	target.EmitData(msgData)

	funcs := []*ir.Function{
		b1.Build(), bSub.Build(), bMul.Build(), bNeg.Build(), bMax.Build(), bPos.Build(), f2,
	}
	for _, f := range funcs {
		compile(target, globals, f)
	}
}
