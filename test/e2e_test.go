package test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/arch/arm64"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/codegen"
	"github.com/ugurkorkmaz/qbe-go/ir"
	"github.com/ugurkorkmaz/qbe-go/opt"
)

type TestCase struct {
	Name     string
	Build    func(b *builder.Builder)
	Expected string
}

func runE2E(t *testing.T, tc TestCase, arg int) {
	b := builder.NewBuilder(tc.Name)
	tc.Build(b)
	f := b.Build()
	f.Exported = true

	target := &arm64.ARM64Target{Apple: false}
	analysis.SSA(f)
	target.Simplify(f)
	opt.DCE(f)
	opt.PhiElim(f)
	codegen.Spill(f, target)
	target.ABI0(f)
	codegen.NewRegAllocator(f, target).Allocate()

	var asmBuf bytes.Buffer
	target.Out = &asmBuf
	target.Emit(f, nil)

	tmpDir := t.TempDir()
	asmFile := filepath.Join(tmpDir, "test.s")
	os.WriteFile(asmFile, asmBuf.Bytes(), 0644)

	wrapperC := fmt.Sprintf(`
#include <stdio.h>
extern int %s(int);
int main() {
    printf("%%d", %s(%d));
    return 0;
}
`, tc.Name, tc.Name, arg)

	wrapperFile := filepath.Join(tmpDir, "wrapper.c")
	os.WriteFile(wrapperFile, []byte(wrapperC), 0644)

	binFile := filepath.Join(tmpDir, "test.bin")
	cmd := exec.Command("cc", wrapperFile, asmFile, "-o", binFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Compile failed: %v\nOutput: %s", err, out)
	}

	out, err := exec.Command(binFile).Output()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	actual := strings.TrimSpace(string(out))
	if actual != tc.Expected {
		t.Errorf("%s: Expected %q, got %q", tc.Name, tc.Expected, actual)
	}
}

func TestArithmetic(t *testing.T) {
	tests := []TestCase{
		{
			Name: "add_simple",
			Build: func(b *builder.Builder) {
				b.Block("start")
				res := b.Add(ir.Kw, b.Param(ir.Kw, "a"), b.Con(20))
				b.Ret(ir.Kw, res)
			},
			Expected: "30",
		},
		{
			Name: "madd_test",
			Build: func(b *builder.Builder) {
				b.Block("start")
				p1 := b.Param(ir.Kw, "a")
				m := b.Mul(ir.Kw, p1, b.Con(5))
				res := b.Add(ir.Kw, m, b.Con(10))
				b.Ret(ir.Kw, res)
			},
			Expected: "60", // 10*5 + 10
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) { runE2E(t, tc, 10) })
	}
}
