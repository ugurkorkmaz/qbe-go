package parser

import (
	"fmt"
	"strconv"

	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

type Parser struct {
	lexer *Lexer
	tok   Token
	
	// Global state
	globals map[string]uint32
	
	// State for current function
	builder *builder.Builder
	locals  map[string]ir.Ref
	blocks  map[string]*ir.Block
	params  []pendingParam
}

type pendingParam struct {
	cls  ir.Class
	name string
}

func NewParser(input string) *Parser {
	p := &Parser{
		lexer:   NewLexer(input),
		globals: make(map[string]uint32),
		locals:  make(map[string]ir.Ref),
		blocks:  make(map[string]*ir.Block),
	}
	p.tok = p.lexer.Lex()
	return p
}

func (p *Parser) expect(kind TokenKind) {
	if p.tok.Kind != kind {
		panic(fmt.Sprintf("line %d: expected %v, got %v (%q)", p.tok.Line, kind, p.tok.Kind, p.tok.Val))
	}
	p.next()
}

func (p *Parser) next() {
	p.tok = p.lexer.Lex()
}

func (p *Parser) Parse() []*ir.Function {
	var funcs []*ir.Function
	for p.tok.Kind != TokEOF {
		switch p.tok.Kind {
		case TokExport:
			p.next()
			funcs = append(funcs, p.parseFunction(true))
		case TokFunction:
			funcs = append(funcs, p.parseFunction(false))
		default:
			p.next()
		}
	}
	return funcs
}

func (p *Parser) GetGlobals() []string {
	names := make([]string, len(p.globals))
	for name, idx := range p.globals {
		names[idx] = name
	}
	return names
}

func (p *Parser) getGlobal(name string) ir.Ref {
	if idx, ok := p.globals[name]; ok {
		return ir.Ref{Kind: ir.RSym, Val: idx}
	}
	idx := uint32(len(p.globals))
	p.globals[name] = idx
	return ir.Ref{Kind: ir.RSym, Val: idx}
}

func (p *Parser) parseFunction(exported bool) *ir.Function {
	p.expect(TokFunction)
	_ = p.parseClass()
	nameTok := p.tok
	p.expect(TokGlobal)
	
	p.builder = builder.NewBuilder(nameTok.Val)
	p.locals = make(map[string]ir.Ref)
	p.blocks = make(map[string]*ir.Block)
	p.params = nil
	
	p.expect(TokLParen)
	for p.tok.Kind != TokRParen {
		pcls := p.parseClass()
		pname := p.tok.Val
		p.expect(TokLocal)
		p.params = append(p.params, pendingParam{pcls, pname})
		if p.tok.Kind == TokComma { p.next() }
	}
	p.expect(TokRParen)
	
	p.expect(TokLBrace)
	for p.tok.Kind != TokRBrace {
		if p.tok.Kind == TokBlock { p.parseBlock() } else { p.parseInstruction() }
	}
	p.expect(TokRBrace)
	
	f := p.builder.Build()
	f.Exported = exported
	return f
}

func (p *Parser) parseClass() ir.Class {
	name := p.tok.Val
	p.expect(TokIdent)
	switch name {
	case "w": return ir.Kw
	case "l": return ir.Kl
	case "s": return ir.Ks
	case "d": return ir.Kd
	case "b": return ir.Kb
	case "h": return ir.Kh
	}
	return ir.Kw
}

func (p *Parser) parseBlock() {
	name := p.tok.Val
	p.expect(TokBlock)
	isFirst := len(p.blocks) == 0
	blk := p.getOrCreateBlock(name)
	p.builder.SetBlock(blk)
	if isFirst {
		for _, pp := range p.params {
			p.locals[pp.name] = p.builder.Param(pp.cls, pp.name)
		}
	}
}

func (p *Parser) parseInstruction() {
	if p.tok.Kind == TokLocal {
		targetName := p.tok.Val
		p.next()
		p.expect(TokEqual)
		cls := p.parseClass()
		opName := p.tok.Val
		p.expect(TokIdent)
		
		if opName == "call" {
			target := p.parseRef()
			p.expect(TokLParen)
			var args []ir.Ref
			for p.tok.Kind != TokRParen {
				args = append(args, p.parseRef())
				if p.tok.Kind == TokComma { p.next() }
			}
			p.expect(TokRParen)
			p.locals[targetName] = p.builder.Call(cls, target, args)
			return
		}

		args := p.parseArgs()
		op := p.lookupOp(opName)
		res := p.builder.Tmp(targetName, cls)
		arg1, arg2, arg3 := ir.Undef, ir.Undef, ir.Undef
		if len(args) > 0 { arg1 = args[0] }
		if len(args) > 1 { arg2 = args[1] }
		if len(args) > 2 { arg3 = args[2] }
		p.builder.Ins(op, cls, res, arg1, arg2, arg3)
		p.locals[targetName] = res
		return
	}
	
	if p.tok.Kind == TokIdent && p.tok.Val == "call" {
		p.next()
		target := p.parseRef()
		p.expect(TokLParen)
		var args []ir.Ref
		for p.tok.Kind != TokRParen {
			args = append(args, p.parseRef())
			if p.tok.Kind == TokComma { p.next() }
		}
		p.expect(TokRParen)
		p.builder.Call(ir.Kx, target, args)
		return
	}

	if p.tok.Kind == TokRet {
		p.next()
		p.builder.Ret(ir.Kw, p.parseRef())
		return
	}
	if p.tok.Kind == TokIdent && p.tok.Val == "jmp" {
		p.next()
		target := p.tok.Val
		p.expect(TokBlock)
		p.builder.Jmp(p.getOrCreateBlock(target))
		return
	}
	if p.tok.Kind == TokIdent && p.tok.Val == "jnz" {
		p.next()
		arg := p.parseRef()
		p.expect(TokComma)
		tname := p.tok.Val
		p.expect(TokBlock)
		p.expect(TokComma)
		fname := p.tok.Val
		p.expect(TokBlock)
		p.builder.Jnz(arg, p.getOrCreateBlock(tname), p.getOrCreateBlock(fname))
		return
	}
	p.next()
}

func (p *Parser) parseArgs() []ir.Ref {
	var args []ir.Ref
	for {
		args = append(args, p.parseRef())
		if p.tok.Kind != TokComma { break }
		p.next()
	}
	return args
}

func (p *Parser) parseRef() ir.Ref {
	switch p.tok.Kind {
	case TokLocal:
		name := p.tok.Val
		p.next()
		return p.locals[name]
	case TokGlobal:
		name := p.tok.Val
		p.next()
		return p.getGlobal(name)
	case TokNumber:
		val, _ := strconv.ParseInt(p.tok.Val, 10, 64)
		p.next()
		return p.builder.Con(uint64(val))
	}
	return ir.Undef
}

func (p *Parser) getOrCreateBlock(name string) *ir.Block {
	if b, ok := p.blocks[name]; ok { return b }
	b := p.builder.Block(name)
	p.blocks[name] = b
	return b
}

func (p *Parser) lookupOp(name string) ir.Opcode {
	switch name {
	case "add": return ir.Oadd
	case "sub": return ir.Osub
	case "mul": return ir.Omul
	case "div": return ir.Odiv
	case "udiv": return ir.Oudiv
	case "rem": return ir.Orem
	case "urem": return ir.Ourem
	case "and": return ir.Oand
	case "or": return ir.Oor
	case "xor": return ir.Oxor
	case "sar": return ir.Osar
	case "shr": return ir.Oshr
	case "shl": return ir.Oshl
	case "load": return ir.Oload
	case "loadub": return ir.Oloadub
	case "loadsb": return ir.Oloadsb
	case "store": return ir.Ostorel
	case "storeb": return ir.Ostoreb
	case "ceqw": return ir.Oceqw
	case "cnew": return ir.Ocnew
	case "csltw": return ir.Ocsltw
	case "cslew": return ir.Ocslew
	case "csgtw": return ir.Ocsgtw
	case "csgew": return ir.Ocsgew
	case "ceql": return ir.Oceql
	case "cnel": return ir.Ocnel
	case "csltl": return ir.Ocsltl
	case "cslel": return ir.Ocslel
	case "csgtl": return ir.Ocsgtl
	case "csgel": return ir.Ocsgel
	case "copy": return ir.Ocopy
	case "extub": return ir.Oextub
	case "extsb": return ir.Oextsb
	case "extuw": return ir.Oextuw
	case "extsw": return ir.Oextsw
	}
	return ir.Onop
}
