package parser_test

import (
	"testing"

	"github.com/ugurkorkmaz/qbe-go/parser"
)

func TestParseSimple(t *testing.T) {
	input := `
export function w $add(w %a, w %b) {
@start
	%c =w add %a, %b
	ret %c
}
`
	p := parser.NewParser(input)
	funcs := p.Parse()

	if len(funcs) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(funcs))
	}

	f := funcs[0]
	if f.Name != "add" {
		t.Errorf("Expected function name 'add', got %q", f.Name)
	}

	if len(f.Blocks) != 1 {
		t.Errorf("Expected 1 block, got %d", len(f.Blocks))
	}
}
