package parser

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

type TokenKind int

const (
	TokEOF TokenKind = iota
	TokError
	
	TokIdent   // Identifier
	TokGlobal  // $name
	TokLocal   // %name
	TokBlock   // @name
	TokNumber  // 123
	TokString  // "abc"
	
	TokEqual   // =
	TokComma   // ,
	TokLParen  // (
	TokRParen  // )
	TokLBrace  // {
	TokRBrace  // }
	TokColon   // :
	
	// Keywords
	TokExport
	TokFunction
	TokType
	TokData
	TokSection
	TokRet
)

type Token struct {
	Kind TokenKind
	Val  string
	Line int
}

type Lexer struct {
	input string
	pos   int
	line  int
}

func NewLexer(input string) *Lexer {
	return &Lexer{input: input, line: 1}
}

func (l *Lexer) next() rune {
	if l.pos >= len(l.input) {
		return -1
	}
	r, size := utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += size
	if r == '\n' {
		l.line++
	}
	return r
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return -1
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.pos:])
	return r
}

func (l *Lexer) Lex() Token {
	l.skipWhitespace()
	
	r := l.next()
	if r == -1 {
		return Token{Kind: TokEOF, Line: l.line}
	}
	
	switch r {
	case '=': return Token{Kind: TokEqual, Val: "=", Line: l.line}
	case ',': return Token{Kind: TokComma, Val: ",", Line: l.line}
	case '(': return Token{Kind: TokLParen, Val: "(", Line: l.line}
	case ')': return Token{Kind: TokRParen, Val: ")", Line: l.line}
	case '{': return Token{Kind: TokLBrace, Val: "{", Line: l.line}
	case '}': return Token{Kind: TokRBrace, Val: "}", Line: l.line}
	case ':': return Token{Kind: TokColon, Val: ":", Line: l.line}
	case '$': return l.lexIdent(TokGlobal)
	case '%': return l.lexIdent(TokLocal)
	case '@': return l.lexIdent(TokBlock)
	case '"': return l.lexString()
	}
	
	if unicode.IsDigit(r) || r == '-' {
		return l.lexNumber(r)
	}
	
	if isIdentStart(r) {
		return l.lexWord(r)
	}
	
	return Token{Kind: TokError, Val: fmt.Sprintf("unexpected character %q", r), Line: l.line}
}

func (l *Lexer) skipWhitespace() {
	for {
		r := l.peek()
		if unicode.IsSpace(r) {
			l.next()
		} else if r == '#' { // Comment
			for l.peek() != '\n' && l.peek() != -1 {
				l.next()
			}
		} else {
			break
		}
	}
}

func (l *Lexer) lexIdent(kind TokenKind) Token {
	start := l.pos
	for isIdentChar(l.peek()) {
		l.next()
	}
	return Token{Kind: kind, Val: l.input[start:l.pos], Line: l.line}
}

func (l *Lexer) lexWord(first rune) Token {
	start := l.pos - utf8.RuneLen(first)
	for isIdentChar(l.peek()) {
		l.next()
	}
	val := l.input[start:l.pos]
	
	switch val {
	case "export": return Token{Kind: TokExport, Val: val, Line: l.line}
	case "function": return Token{Kind: TokFunction, Val: val, Line: l.line}
	case "type": return Token{Kind: TokType, Val: val, Line: l.line}
	case "data": return Token{Kind: TokData, Val: val, Line: l.line}
	case "section": return Token{Kind: TokSection, Val: val, Line: l.line}
	case "ret": return Token{Kind: TokRet, Val: val, Line: l.line}
	}
	
	return Token{Kind: TokIdent, Val: val, Line: l.line}
}

func (l *Lexer) lexNumber(first rune) Token {
	start := l.pos - utf8.RuneLen(first)
	for unicode.IsDigit(l.peek()) {
		l.next()
	}
	return Token{Kind: TokNumber, Val: l.input[start:l.pos], Line: l.line}
}

func (l *Lexer) lexString() Token {
	start := l.pos
	for {
		r := l.next()
		if r == '"' {
			return Token{Kind: TokString, Val: l.input[start : l.pos-1], Line: l.line}
		}
		if r == -1 {
			return Token{Kind: TokError, Val: "unterminated string", Line: l.line}
		}
	}
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_' || r == '.'
}

func isIdentChar(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r)
}
