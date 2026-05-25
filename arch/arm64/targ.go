package arm64

import (
	"fmt"
	"io"
	"os"
)

// ARM64 specific register constants (AArch64)
const (
	X0 = iota
	X1
	X2
	X3
	X4
	X5
	X6
	X7
	X8
	X9
	X10
	X11
	X12
	X13
	X14
	X15
	X16
	X17
	X18
	X19
	X20
	X21
	X22
	X23
	X24
	X25
	X26
	X27
	X28
	X29 // FP
	X30 // LR
	SP  // Stack Pointer
	V0  // Floating point 0
	V1
	V2
	V3
	V4
	V5
	V6
	V7
	V8
	V9
	V10
	V11
	V12
	V13
	V14
	V15
	V16
	V17
	V18
	V19
	V20
	V21
	V22
	V23
	V24
	V25
	V26
	V27
	V28
	V29
	V30
	V31
)

var gprNames = []string{
	"x0", "x1", "x2", "x3", "x4", "x5", "x6", "x7",
	"x8", "x9", "x10", "x11", "x12", "x13", "x14", "x15",
	"x16", "x17", "x18", "x19", "x20", "x21", "x22", "x23",
	"x24", "x25", "x26", "x27", "x28", "x29", "x30", "sp",
}

var gpr32Names = []string{
	"w0", "w1", "w2", "w3", "w4", "w5", "w6", "w7",
	"w8", "w9", "w10", "w11", "w12", "w13", "w14", "w15",
	"w16", "w17", "w18", "w19", "w20", "w21", "w22", "w23",
	"w24", "w25", "w26", "w27", "w28", "w29", "w30", "wsp",
}

// System V / Apple ARM64 ABI parameter registers
var paramGPR = []int{X0, X1, X2, X3, X4, X5, X6, X7}
var paramFPR = []int{V0, V1, V2, V3, V4, V5, V6, V7}

type ARM64Target struct {
	Apple          bool
	CurrentGlobals []string
	Out            io.Writer
}

func (t *ARM64Target) w() io.Writer {
	if t.Out != nil {
		return t.Out
	}
	return os.Stdout
}

func (t *ARM64Target) Name() string {
	if t.Apple {
		return "arm64_apple"
	}
	return "arm64"
}

func (t *ARM64Target) GPR0() int { return X0 }
func (t *ARM64Target) NGPR() int { return 31 }
func (t *ARM64Target) FPR0() int { return V0 }
func (t *ARM64Target) NFPR() int { return 32 }

func (t *ARM64Target) RegName(r int) string {
	if r >= 0 && r < 32 {
		return gprNames[r]
	}
	if r >= V0 && r < V0+32 {
		return fmt.Sprintf("d%d", r-V0)
	}
	return fmt.Sprintf("reg%d", r)
}
