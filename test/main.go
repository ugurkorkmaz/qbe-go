package main

import (
	"fmt"
	"math"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/arch/arm64"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/codegen"
	"github.com/ugurkorkmaz/qbe-go/ir"
	"github.com/ugurkorkmaz/qbe-go/opt"
)

func compile(target *arm64.ARM64Target, globals []string, f *ir.Function) {
	analysis.SSA(f)
	opt.PhiElim(f)
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
		"fadd",        // 10
		"fact",        // 11
		"clamp",       // 12
		"abs_diff",    // 13
	}

	// --- add(w %a, w %b) ---
	b1 := builder.NewBuilder("add")
	b1.Block("start")
	b1.Ret(ir.Kw, b1.Add(ir.Kw, b1.Param(ir.Kw, "a"), b1.Param(ir.Kw, "b")))

	// --- sub(w %a, w %b) ---
	bSub := builder.NewBuilder("sub")
	bSub.Block("start")
	bSub.Ret(ir.Kw, bSub.Sub(ir.Kw, bSub.Param(ir.Kw, "a"), bSub.Param(ir.Kw, "b")))

	// --- mul(w %a, w %b) ---
	bMul := builder.NewBuilder("mul")
	bMul.Block("start")
	bMul.Ret(ir.Kw, bMul.Mul(ir.Kw, bMul.Param(ir.Kw, "a"), bMul.Param(ir.Kw, "b")))

	// --- neg_w(w %a) ---
	bNeg := builder.NewBuilder("neg_w")
	bNeg.Block("start")
	bNeg.Ret(ir.Kw, bNeg.Neg(ir.Kw, bNeg.Param(ir.Kw, "a")))

	// --- max_w(w %a, w %b) ---
	bMax := builder.NewBuilder("max_w")
	startBlk := bMax.Block("start")
	thenBlk := bMax.Block("then")
	elseBlk := bMax.Block("else")
	bMax.SetBlock(startBlk)
	mxa := bMax.Param(ir.Kw, "a")
	mxb := bMax.Param(ir.Kw, "b")
	bMax.Jnz(bMax.Compare(ir.Ocsgtw, ir.Kw, mxa, mxb), thenBlk, elseBlk)
	bMax.SetBlock(thenBlk)
	bMax.Ret(ir.Kw, mxa)
	bMax.SetBlock(elseBlk)
	bMax.Ret(ir.Kw, mxb)

	// --- is_positive(w %a) ---
	bPos := builder.NewBuilder("is_positive")
	bPos.Block("start")
	bPos.Ret(ir.Kw, bPos.Compare(ir.Ocsgtw, ir.Kw, bPos.Param(ir.Kw, "a"), ir.NewInt(0)))

	// --- Test 6: fadd(d %a, d %b) → %a + %b ---
	bFadd := builder.NewBuilder("fadd")
	bFadd.Block("start")
	bFadd.Ret(ir.Kd, bFadd.Add(ir.Kd, bFadd.Param(ir.Kd, "a"), bFadd.Param(ir.Kd, "b")))

	// --- Test 7: fact(w %n) → n! (recursive) ---
	bFact := builder.NewBuilder("fact")
	factStart := bFact.Block("start")
	factBase := bFact.Block("base")
	factRec := bFact.Block("recurse")
	bFact.SetBlock(factStart)
	fn := bFact.Param(ir.Kw, "n")
	bFact.Jnz(bFact.Compare(ir.Ocslew, ir.Kw, fn, ir.NewInt(1)), factBase, factRec)
	bFact.SetBlock(factBase)
	bFact.Ret(ir.Kw, ir.NewInt(1))
	bFact.SetBlock(factRec)
	fn1 := bFact.Sub(ir.Kw, fn, ir.NewInt(1))
	fr := bFact.Call(ir.Kw, ir.Ref{Kind: ir.RSym, Val: 11}, []ir.Ref{fn1})
	bFact.Ret(ir.Kw, bFact.Mul(ir.Kw, fn, fr))

	// --- Test 8: clamp(w %x, w %lo, w %hi) ---
	bClamp := builder.NewBuilder("clamp")
	clStart := bClamp.Block("start")
	clRetLo := bClamp.Block("ret_lo")
	clCheckHi := bClamp.Block("check_hi")
	clRetHi := bClamp.Block("ret_hi")
	clRetX := bClamp.Block("ret_x")
	bClamp.SetBlock(clStart)
	cx := bClamp.Param(ir.Kw, "x")
	clo := bClamp.Param(ir.Kw, "lo")
	chi := bClamp.Param(ir.Kw, "hi")
	bClamp.Jnz(bClamp.Compare(ir.Ocsltw, ir.Kw, cx, clo), clRetLo, clCheckHi)
	bClamp.SetBlock(clRetLo)
	bClamp.Ret(ir.Kw, clo)
	bClamp.SetBlock(clCheckHi)
	bClamp.Jnz(bClamp.Compare(ir.Ocsgtw, ir.Kw, cx, chi), clRetHi, clRetX)
	bClamp.SetBlock(clRetHi)
	bClamp.Ret(ir.Kw, chi)
	bClamp.SetBlock(clRetX)
	bClamp.Ret(ir.Kw, cx)

	// --- Test 9: abs_diff(w %a, w %b) → |a - b| ---
	bAbs := builder.NewBuilder("abs_diff")
	absStart := bAbs.Block("start")
	absThen := bAbs.Block("then")
	absElse := bAbs.Block("else")
	bAbs.SetBlock(absStart)
	aa := bAbs.Param(ir.Kw, "a")
	ab := bAbs.Param(ir.Kw, "b")
	bAbs.Jnz(bAbs.Compare(ir.Ocsgtw, ir.Kw, aa, ab), absThen, absElse)
	bAbs.SetBlock(absThen)
	bAbs.Ret(ir.Kw, bAbs.Sub(ir.Kw, aa, ab))
	bAbs.SetBlock(absElse)
	bAbs.Ret(ir.Kw, bAbs.Sub(ir.Kw, ab, aa))

	// --- main() ---
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
	faddSym := ir.Ref{Kind: ir.RSym, Val: 10}
	factSym := ir.Ref{Kind: ir.RSym, Val: 11}
	clampSym := ir.Ref{Kind: ir.RSym, Val: 12}
	absDiffSym := ir.Ref{Kind: ir.RSym, Val: 13}

	msgAddr := b2.Copy(ir.Kl, msgSym)
	b2.Call(ir.Kw, printStrSym, []ir.Ref{msgAddr})

	// add(10, 20) → 30
	b2.Call(ir.Kw, printIntSym, []ir.Ref{
		b2.Call(ir.Kw, ir.Ref{Kind: ir.RSym, Val: 0}, []ir.Ref{b2.Con(10), b2.Con(20)}),
	})

	// print_float(3.14159)
	b2.Ins(ir.Oarg, ir.Kd, ir.Undef, b2.Con(math.Float64bits(3.14159)), ir.Undef)
	b2.Ins(ir.Ocall, ir.Kw, ir.Undef, printFloatSym, ir.Undef)

	// sub(15, 6) → 9
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, subSym, []ir.Ref{b2.Con(15), b2.Con(6)})})

	// mul(6, 7) → 42
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, mulSym, []ir.Ref{b2.Con(6), b2.Con(7)})})

	// neg_w(5) → -5
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, negSym, []ir.Ref{b2.Con(5)})})

	// max_w(3, 7) → 7
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, maxSym, []ir.Ref{b2.Con(3), b2.Con(7)})})

	// is_positive(-1) → 0, is_positive(5) → 1
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, isPosSym, []ir.Ref{b2.Con(^uint64(0))})})
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, isPosSym, []ir.Ref{b2.Con(5)})})

	// fadd(1.5, 2.5) → print_float(4.0)
	fa := b2.Con(math.Float64bits(1.5))
	fb := b2.Con(math.Float64bits(2.5))
	b2.Ins(ir.Oarg, ir.Kd, ir.Undef, fa, ir.Undef)
	b2.Ins(ir.Oarg, ir.Kd, ir.Undef, fb, ir.Undef)
	rfadd := b2.Tmp("", ir.Kd)
	b2.Ins(ir.Ocall, ir.Kd, rfadd, faddSym, ir.Undef)
	b2.Ins(ir.Oarg, ir.Kd, ir.Undef, rfadd, ir.Undef)
	b2.Ins(ir.Ocall, ir.Kw, ir.Undef, printFloatSym, ir.Undef)

	// fact(5) → 120
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, factSym, []ir.Ref{b2.Con(5)})})

	// clamp(15,0,10)→10, clamp(-5,0,10)→0 (but -5 as uint overflows, use NewInt), clamp(5,0,10)→5
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, clampSym, []ir.Ref{b2.Con(15), ir.NewInt(0), b2.Con(10)})})
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, clampSym, []ir.Ref{ir.NewInt(-5), ir.NewInt(0), b2.Con(10)})})
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, clampSym, []ir.Ref{b2.Con(5), ir.NewInt(0), b2.Con(10)})})

	// abs_diff(3,7)→4, abs_diff(10,4)→6
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, absDiffSym, []ir.Ref{b2.Con(3), b2.Con(7)})})
	b2.Call(ir.Kw, printIntSym, []ir.Ref{b2.Call(ir.Kw, absDiffSym, []ir.Ref{b2.Con(10), b2.Con(4)})})

	b2.Ret(ir.Kw, ir.NewInt(0))
	f2 := b2.Build()
	f2.Exported = true

	msgData := &ir.Data{
		Label:    "msg",
		Exported: false,
		Items:    []ir.DataItem{{Type: "b", String: "Hello world from qbe-go!\n"}},
	}

	target.EmitData(msgData)

	funcs := []*ir.Function{
		b1.Build(), bSub.Build(), bMul.Build(), bNeg.Build(),
		bMax.Build(), bPos.Build(),
		bFadd.Build(), bFact.Build(), bClamp.Build(), bAbs.Build(),
		f2,
	}
	for _, f := range funcs {
		compile(target, globals, f)
	}
}
