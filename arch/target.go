package arch

import "github.com/ugurkorkmaz/qbe-go/ir"

// Target represents an architecture-specific backend.
type Target interface {
	Name() string
	GPR0() int
	NGPR() int
	FPR0() int
	NFPR() int
	RegName(r int) string
	ABI0(f *ir.Function)
	Emit(f *ir.Function, globals []string) error
	EmitData(d *ir.Data)
}
