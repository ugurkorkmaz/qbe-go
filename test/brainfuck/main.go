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

// BFCompiler maps Brainfuck code to qbe-go IR
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
	c.ptr = b.Param(ir.Kl, "tape")
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
			c.ptr = b.Add(ir.Kl, c.ptr, b.Con(1))
		case '<':
			c.ptr = b.Sub(ir.Kl, c.ptr, b.Con(1))
		case '+':
			val := b.LoadUB(ir.Kw, c.ptr)
			inc := b.Add(ir.Kw, val, b.Con(1))
			b.Ins(ir.Ostoreb, ir.Kw, ir.Undef, inc, c.ptr)
		case '-':
			val := b.LoadUB(ir.Kw, c.ptr)
			dec := b.Sub(ir.Kw, val, b.Con(1))
			b.Ins(ir.Ostoreb, ir.Kw, ir.Undef, dec, c.ptr)
		case '.':
			val := b.LoadUB(ir.Kw, c.ptr)
			b.Call(ir.Kw, putcharSym, []ir.Ref{val})
		case ',':
			res := b.Call(ir.Kw, getcharSym, nil)
			b.Ins(ir.Ostoreb, ir.Kw, ir.Undef, res, c.ptr)
		case '[':
			start := b.Block(fmt.Sprintf("loop_start_%d", c.pos))
			body := b.Block(fmt.Sprintf("loop_body_%d", c.pos))
			end := b.Block(fmt.Sprintf("loop_end_%d", c.pos))
			
			// Previous block jumps to start
			prev := b.Func.Blocks[len(b.Func.Blocks)-4] // start, body, end just added
			// Actually builder.Block sets b.cur. Let's be careful.
			// The block before '[' was the current block.
			
			// To correctly link blocks:
			// 1. Link previous block to 'start'
			prev.Jmp = ir.Jump{Type: ir.Jjmp}
			prev.S1 = start
			
			// 2. start: if (val != 0) body else end
			b.SetBlock(start)
			val := b.LoadUB(ir.Kw, c.ptr)
			b.Jnz(val, body, end)
			
			// 3. body: current
			b.SetBlock(body)
			stack = append(stack, loop{start, end})
		case ']':
			if len(stack) == 0 {
				log.Fatal("Unmatched ']'")
			}
			l := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			
			// Body block jumps back to start
			b.Jmp(l.start)
			
			// New code goes to 'end'
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

	analysis.SSA(f)
	codegen.Spill(f, target)
	target.ABI0(f)
	codegen.NewRegAllocator(f, target).Allocate()

	if err := target.Emit(f, globals); err != nil {
		log.Fatal(err)
	}
}
