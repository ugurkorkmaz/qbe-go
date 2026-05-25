package arm64_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/arch/arm64"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/codegen"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

func compile(t *testing.T, f *ir.Function) string {
	t.Helper()
	tgt := &arm64.ARM64Target{Apple: false}
	var buf bytes.Buffer
	tgt.Out = &buf
	analysis.SSA(f)
	codegen.Spill(f, tgt)
	tgt.ABI0(f)
	codegen.NewRegAllocator(f, tgt).Allocate()
	if err := tgt.Emit(f, nil); err != nil {
		t.Fatalf("emit error: %v", err)
	}
	return buf.String()
}

func assertContains(t *testing.T, asm, want string) {
	t.Helper()
	if !strings.Contains(asm, want) {
		t.Errorf("expected %q in output:\n%s", want, asm)
	}
}

func assertNotContains(t *testing.T, asm, want string) {
	t.Helper()
	if strings.Contains(asm, want) {
		t.Errorf("unexpected %q in output:\n%s", want, asm)
	}
}

func TestNegInt(t *testing.T) {
	b := builder.NewBuilder("neg_int")
	b.Block("start")
	a := b.Param(ir.Kw, "a")
	r := b.Neg(ir.Kw, a)
	b.Ret(ir.Kw, r)
	assertContains(t, compile(t, b.Build()), "neg ")
}

func TestNegFloat(t *testing.T) {
	b := builder.NewBuilder("neg_float")
	b.Block("start")
	a := b.Param(ir.Ks, "a")
	r := b.Neg(ir.Ks, a)
	b.Ret(ir.Ks, r)
	assertContains(t, compile(t, b.Build()), "fneg ")
}

func TestLoadSB(t *testing.T) {
	b := builder.NewBuilder("loadsb")
	b.Block("start")
	p := b.Param(ir.Kl, "ptr")
	r := b.LoadSB(ir.Kw, p)
	b.Ret(ir.Kw, r)
	assertContains(t, compile(t, b.Build()), "ldrsb ")
}

func TestLoadUB(t *testing.T) {
	b := builder.NewBuilder("loadub")
	b.Block("start")
	p := b.Param(ir.Kl, "ptr")
	r := b.LoadUB(ir.Kw, p)
	b.Ret(ir.Kw, r)
	assertContains(t, compile(t, b.Build()), "ldrb ")
}

func TestLoadSH(t *testing.T) {
	b := builder.NewBuilder("loadsh")
	b.Block("start")
	p := b.Param(ir.Kl, "ptr")
	r := b.LoadSH(ir.Kw, p)
	b.Ret(ir.Kw, r)
	assertContains(t, compile(t, b.Build()), "ldrsh ")
}

func TestLoadUH(t *testing.T) {
	b := builder.NewBuilder("loaduh")
	b.Block("start")
	p := b.Param(ir.Kl, "ptr")
	r := b.LoadUH(ir.Kw, p)
	b.Ret(ir.Kw, r)
	assertContains(t, compile(t, b.Build()), "ldrh ")
}

func TestLoadSW(t *testing.T) {
	b := builder.NewBuilder("loadsw")
	b.Block("start")
	p := b.Param(ir.Kl, "ptr")
	r := b.LoadSW(p)
	b.Ret(ir.Kl, r)
	assertContains(t, compile(t, b.Build()), "ldrsw ")
}

func TestLoadUW(t *testing.T) {
	b := builder.NewBuilder("loaduw")
	b.Block("start")
	p := b.Param(ir.Kl, "ptr")
	r := b.LoadUW(p)
	b.Ret(ir.Kl, r)
	asm := compile(t, b.Build())
	assertContains(t, asm, "ldr w")
}

func TestExtSB(t *testing.T) {
	b := builder.NewBuilder("extsb")
	b.Block("start")
	a := b.Param(ir.Kw, "a")
	r := b.ExtSB(ir.Kw, a)
	b.Ret(ir.Kw, r)
	assertContains(t, compile(t, b.Build()), "sxtb ")
}

func TestExtUB(t *testing.T) {
	b := builder.NewBuilder("extub")
	b.Block("start")
	a := b.Param(ir.Kw, "a")
	r := b.ExtUB(ir.Kw, a)
	b.Ret(ir.Kw, r)
	assertContains(t, compile(t, b.Build()), "uxtb ")
}

func TestExtSH(t *testing.T) {
	b := builder.NewBuilder("extsh")
	b.Block("start")
	a := b.Param(ir.Kw, "a")
	r := b.ExtSH(ir.Kw, a)
	b.Ret(ir.Kw, r)
	assertContains(t, compile(t, b.Build()), "sxth ")
}

func TestExtUH(t *testing.T) {
	b := builder.NewBuilder("extuh")
	b.Block("start")
	a := b.Param(ir.Kw, "a")
	r := b.ExtUH(ir.Kw, a)
	b.Ret(ir.Kw, r)
	assertContains(t, compile(t, b.Build()), "uxth ")
}

func TestExtSW(t *testing.T) {
	b := builder.NewBuilder("extsw")
	b.Block("start")
	a := b.Param(ir.Kw, "a")
	r := b.ExtSW(a)
	b.Ret(ir.Kl, r)
	assertContains(t, compile(t, b.Build()), "sxtw ")
}

func TestExtUW(t *testing.T) {
	b := builder.NewBuilder("extuw")
	b.Block("start")
	a := b.Param(ir.Kw, "a")
	r := b.ExtUW(a)
	b.Ret(ir.Kl, r)
	assertContains(t, compile(t, b.Build()), "mov w")
}

func TestSDiv(t *testing.T) {
	bld := builder.NewBuilder("sdiv_test")
	bld.Block("start")
	a := bld.Param(ir.Kw, "a")
	p2 := bld.Param(ir.Kw, "b")
	to := bld.Tmp("", ir.Kw)
	bld.Ins(ir.Odiv, ir.Kw, to, a, p2)
	bld.Ret(ir.Kw, to)
	asm := compile(t, bld.Build())
	assertContains(t, asm, "sdiv ")
	assertNotContains(t, asm, "\tdiv ")
}

func TestUnsignedCmp(t *testing.T) {
	tests := []struct {
		name string
		op   ir.Opcode
		want string
	}{
		{"ugew", ir.Ocugew, "hs"},
		{"ugtw", ir.Ocugtw, "hi"},
		{"ulew", ir.Oculew, "ls"},
		{"ultw", ir.Ocultw, "lo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := builder.NewBuilder(tc.name)
			b.Block("start")
			a := b.Param(ir.Kw, "a")
			p := b.Param(ir.Kw, "b")
			r := b.Compare(tc.op, ir.Kw, a, p)
			b.Ret(ir.Kw, r)
			assertContains(t, compile(t, b.Build()), tc.want)
		})
	}
}

func TestSignedDiv(t *testing.T) {
	b := builder.NewBuilder("sdiv_check")
	b.Block("start")
	a := b.Param(ir.Kw, "a")
	p := b.Param(ir.Kw, "b")
	r := b.Compare(ir.Ocugew, ir.Kw, a, p)
	b.Ret(ir.Kw, r)
	asm := compile(t, b.Build())
	assertContains(t, asm, "cset")
}
