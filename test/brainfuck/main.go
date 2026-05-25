package main

import (
	"fmt"
	"log"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/arch/arm64"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/codegen"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

type BFCompiler struct {
	builder *builder.Builder
	code    string
	pos     int
	ptr     ir.Ref
}

func NewBFCompiler(name string, bfCode string) *BFCompiler {
	return &BFCompiler{
		builder: builder.NewBuilder(name),
		code:    bfCode,
	}
}

func (c *BFCompiler) Compile() *ir.Function {
	b := c.builder
	b.Block("start")
	
	// Use a stack slot to store the tape pointer to avoid SSA complexities in nested loops
	ptrSlot := b.Func.NewTmp("ptr_addr", ir.Kl)
	b.Ins(ir.Oalloc8, ir.Kl, ptrSlot, ir.Undef, ir.Undef)
	
	// Initialize stack slot with starting pointer (X0)
	tapeArg := b.Param(ir.Kl, "tape")
	b.Ins(ir.Ostorel, ir.Kl, ir.Undef, tapeArg, ptrSlot)
	
	c.ptr = ptrSlot // Now c.ptr is the ADDRESS of the pointer
	c.genIR()
	
	b.Ret(ir.Kw, b.Con(0))
	
	f := b.Build()
	f.Exported = true
	return f
}

func (c *BFCompiler) genIR() {
	b := c.builder
	type loop struct {
		start *ir.Block
		end   *ir.Block
	}
	var stack []loop

	putcharSym := ir.Ref{Kind: ir.RSym, Val: 0}
	getcharSym := ir.Ref{Kind: ir.RSym, Val: 1}

	for c.pos < len(c.code) {
		char := c.code[c.pos]
		c.pos++

		switch char {
		case '>':
			addr := b.Load(ir.Kl, c.ptr)
			newAddr := b.Add(ir.Kl, addr, b.Con(1))
			b.Ins(ir.Ostorel, ir.Kl, ir.Undef, newAddr, c.ptr)
		case '<':
			addr := b.Load(ir.Kl, c.ptr)
			newAddr := b.Sub(ir.Kl, addr, b.Con(1))
			b.Ins(ir.Ostorel, ir.Kl, ir.Undef, newAddr, c.ptr)
		case '+':
			addr := b.Load(ir.Kl, c.ptr)
			val := b.LoadUB(ir.Kw, addr)
			inc := b.Add(ir.Kw, val, b.Con(1))
			b.Ins(ir.Ostoreb, ir.Kw, ir.Undef, inc, addr)
		case '-':
			addr := b.Load(ir.Kl, c.ptr)
			val := b.LoadUB(ir.Kw, addr)
			dec := b.Sub(ir.Kw, val, b.Con(1))
			b.Ins(ir.Ostoreb, ir.Kw, ir.Undef, dec, addr)
		case '.':
			addr := b.Load(ir.Kl, c.ptr)
			val := b.LoadUB(ir.Kw, addr)
			b.Call(ir.Kw, putcharSym, []ir.Ref{val})
		case ',':
			addr := b.Load(ir.Kl, c.ptr)
			res := b.Call(ir.Kw, getcharSym, nil)
			b.Ins(ir.Ostoreb, ir.Kw, ir.Undef, res, addr)
		case '[':
			start := b.Block(fmt.Sprintf("loop_start_%d", c.pos))
			body := b.Block(fmt.Sprintf("loop_body_%d", c.pos))
			end := b.Block(fmt.Sprintf("loop_end_%d", c.pos))
			
			prev := b.Func.Blocks[len(b.Func.Blocks)-4]
			prev.Jmp = ir.Jump{Type: ir.Jjmp, Arg: ir.Undef}
			prev.S1 = start
			
			b.SetBlock(start)
			addr := b.Load(ir.Kl, c.ptr)
			val := b.LoadUB(ir.Kw, addr)
			b.Jnz(val, body, end)
			
			b.SetBlock(body)
			stack = append(stack, loop{start, end})
		case ']':
			if len(stack) == 0 { log.Fatal("Unmatched ']'") }
			l := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			b.Jmp(l.start)
			b.SetBlock(l.end)
		}
	}
}

func main() {
	bf := `++++++++[>++++[>++>+++>+++>+<<<<-]>+>+>->>+[<]<-]>>.>---.+++++++..+++.>>.<-.<.+++.------.--------.>>+.>++.`
	comp := NewBFCompiler("bf_main", bf)
	f := comp.Compile()

	target := &arm64.ARM64Target{Apple: false}
	globals := []string{"putchar", "getchar"}

	analysis.SSA(f) // Still run SSA for basic analysis
	codegen.Spill(f, target)
	target.ABI0(f)
	codegen.NewRegAllocator(f, target).Allocate()

	if err := target.Emit(f, globals); err != nil {
		log.Fatal(err)
	}
}
