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

func main() {
	target := &arm64.ARM64Target{Apple: false}

	// Function $add(w %a, w %b)
	b1 := builder.NewBuilder("add")
	b1.Block("start")
	p1 := b1.Param(ir.Kw, "a")
	p2 := b1.Param(ir.Kw, "b")
	res1 := b1.Add(ir.Kw, p1, p2)
	b1.Ret(ir.Kw, res1)
	f1 := b1.Build()

	// Function $main()
	b2 := builder.NewBuilder("main")
	b2.Block("start")

	// Create string constant for printf format: "Result: %d\n"
	// We use a global symbol (index 2) which emit.go will handle specially
	// fmtStr := ir.Ref{Kind: ir.RSym, Val: 2} // Not used for print_int

	// Call printf(fmt, 30)
	// We need to declare printf simply. In QBE IR: %r =w call $printf(l %fmt, ..., w 30)
	// Symbol 1: _print_int (from C)
	printIntSym := ir.Ref{Kind: ir.RSym, Val: 1}
	// Symbol 2: _print_str (from C)
	printStrSym := ir.Ref{Kind: ir.RSym, Val: 2}
	// Symbol 3: _msg (Data label)
	msgSym := ir.Ref{Kind: ir.RSym, Val: 3}

	// 1. Call print_str(msg)
	// We need to load the address of the message.
	// In QBE IR: %t =l copy $msg; call $print_str(l %t)
	// Our builder.Copy handles address loading for RSym automatically in Emit.
	msgAddr := b2.Copy(ir.Kl, msgSym)
	b2.Call(ir.Kw, printStrSym, []ir.Ref{msgAddr})

	// 2. Call print_int(30)
	val := b2.Con(30)
	b2.Call(ir.Kw, printIntSym, []ir.Ref{val})

	// Symbol 4: _print_float
	printFloatSym := ir.Ref{Kind: ir.RSym, Val: 4}

	// 3. Call print_float(3.14159)
	fval := b2.Con(math.Float64bits(3.14159))
	// Manual Call to ensure Class is Double (Kd)
	// builder.Call heuristic might guess Kl (Int64) for large constants
	b2.Ins(ir.Oarg, ir.Kd, ir.Undef, fval, ir.Undef)
	b2.Ins(ir.Ocall, ir.Kw, ir.Undef, printFloatSym, ir.Undef)

	b2.Ret(ir.Kw, val)
	f2 := b2.Build()
	f2.Exported = true // Make main visible to linker

	// Define Data Section
	msgData := &ir.Data{
		Label:    "msg",
		Exported: false,
		Items: []ir.DataItem{
			{Type: "b", String: "Hello world from qbe-go!\n"},
		},
	}

	// Compile and emit everything
	globals := []string{"add", "print_int", "print_str", "msg", "print_float"}

	// Emit Data first (good practice)
	target.EmitData(msgData)

	for _, f := range []*ir.Function{f1, f2} {
		analysis.SSA(f)
		codegen.Spill(f, target) // Spill analysis + rewrite
		target.ABI0(f)
		codegen.NewRegAllocator(f, target).Allocate()

		if err := target.Emit(f, globals); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}
