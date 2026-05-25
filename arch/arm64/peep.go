package arm64

import (
	"github.com/ugurkorkmaz/qbe-go/ir"
)

// Simplify performs ARM64-specific peephole optimizations.
func (t *ARM64Target) Simplify(f *ir.Function) {
	// Branch Fusion is handled during Emission (PendingCmp)
}
