package main

import (
	"fmt"
	"log"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/arch/arm64"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/codegen"
	"github.com/ugurkorkmaz/qbe-go/ir"
	"github.com/ugurkorkmaz/qbe-go/opt"
)

type BFCompiler struct {
	builder *builder.Builder
	code    string
	pos     int
	ptr     ir.Ref // This will now hold the address of the global tape
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
	
	// Use the global 'tape' symbol directly
	tapeSym := ir.Ref{Kind: ir.RSym, Val: 2} // msg=0, putchar=1, tape=2
	c.ptr = b.Func.NewTmp("ptr", ir.Kl)
	b.Ins(ir.Ocopy, ir.Kl, c.ptr, tapeSym, ir.Undef)
	
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
		phi   ir.Ref
	}
	var stack []loop

	putcharSym := ir.Ref{Kind: ir.RSym, Val: 0}
	// getcharSym := ir.Ref{Kind: ir.RSym, Val: 1} // not used in Hello World

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
		case '[':
			start := b.Block(fmt.Sprintf("loop_start_%d", c.pos))
			body := b.Block(fmt.Sprintf("loop_body_%d", c.pos))
			end := b.Block(fmt.Sprintf("loop_end_%d", c.pos))
			
			prev := b.Func.Blocks[len(b.Func.Blocks)-4]
			prev.Jmp = ir.Jump{Type: ir.Jjmp, Arg: ir.Undef}
			prev.S1 = start
			
			// SSA: The pointer is a Phi node
			phiTo := b.Func.NewTmp("ptr_phi", ir.Kl)
			phi := &ir.Phi{
				To:  phiTo,
				Cls: ir.Kl,
				Args: []ir.Ref{c.ptr},
				Blks: []*ir.Block{prev},
			}
			start.Phis = append(start.Phis, phi)
			c.ptr = phiTo
			
			b.SetBlock(start)
			val := b.LoadUB(ir.Kw, c.ptr)
			b.Jnz(val, body, end)
			
			b.SetBlock(body)
			stack = append(stack, loop{start, end, phiTo})
		case ']':
			if len(stack) == 0 { log.Fatal("Unmatched ']'") }
			l := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			
			bodyBlk := b.Func.Blocks[len(b.Func.Blocks)-1]
			b.Jmp(l.start)
			
			for _, p := range l.start.Phis {
				if p.To == l.phi {
					p.Args = append(p.Args, c.ptr)
					p.Blks = append(p.Blks, bodyBlk)
				}
			}
			c.ptr = l.phi 
			b.SetBlock(l.end)
		}
	}
}

func main() {
	bf := `++++++++[>++++[>++>+++>+++>+<<<<-]>+>+>->>+[<]<-]>>.>---.+++++++..+++.>>.<-.<.+++.------.--------.>>+.>++.`
	comp := NewBFCompiler("bf_main", bf)
	f := comp.Compile()

	target := &arm64.ARM64Target{Apple: false}
	globals := []string{"putchar", "getchar", "tape"}

	// analysis.SSA(f) // Disabled: BF compiler generates SSA IR manually
	// Full Pipeline
	analysis.VerifySSA(f)
	opt.PhiElim(f)
	codegen.Spill(f, target)
	target.ABI0(f)
	codegen.NewRegAllocator(f, target).Allocate()

	// Emit assembly
	if err := target.Emit(f, globals); err != nil { log.Fatal(err) }
	
	// Emit the global tape memory
	tapeData := &ir.Data{
		Label: "tape",
		Exported: false,
		Items: []ir.DataItem{
			{Type: "z", Value: 30000}, // 30,000 bytes of zeros
		},
	}
	target.EmitData(tapeData)
}
