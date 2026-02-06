# QBE-Go

**QBE-Go** is a pure Go implementation of the [QBE Compiler Backend](https://c9x.me/compile/). 
It provides a lightweight, fast, and modular compiler backend designed primarily for the **Sand Programming Language**.

Unlike LLVM or GCC, QBE aims for simplicity (70% of the performance with 10% of the code size). This port brings that philosophy to the Go ecosystem, enabling easy integration with Go-based compilers without CGO dependencies.

## Key Features

- **Pure Go**: No C dependencies. Easy to hack, easy to cross-compile.
- **ARM64 Support**: Fully optimized backend for Apple Silicon (M1/M2/M3) and other ARM64 platforms.
  - Implements the complete procedure call standard (AAPCS64), including HFA (Homogeneous Floating-point Aggregates).
- **Advanced Code Generation**:
  - **Register Allocation**: Linear Scan allocator with **Copy Coalescing** and **Register Hinting**.
  - **Instruction Selection**: Generates native ARM64 instructions including `madd` candidates (future).
  - **Stack Management**: Safe and ABI-compliant stack frame setup/teardown.
- **Optimizations**:
  - **SSA-Based Analysis**: Dominator Tree, Dominance Frontiers, Phi placement.
  - **DCE (Dead Code Elimination)**: Aggressive iterative dead code removal.
  - **GVN (Global Value Numbering)**: Value deduction and redundancy elimination.
- **Builder API**: A fluent Go API to construct IR programmatically.

## Architecture

The project is structured as a library:
- **`ir`**: Intermediate Representation definitions (Instructions, Blocks, Types).
- **`builder`**: Fluent API for generating IR.
- **`analysis`**: SSA construction, Liveness analysis, Control Flow Graph (CFG).
- **`opt`**: Optimization passes (DCE, GVN, etc.).
- **`codegen`**: Register allocation, Spill handling, Stack layout.
- **`arch`**: Architecture-specific implementations (currently `arm64`).

## Usage Example

```go
package main

import (
	"log"
	
	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/arch/arm64"
	"github.com/ugurkorkmaz/qbe-go/builder"
	"github.com/ugurkorkmaz/qbe-go/codegen"
	"github.com/ugurkorkmaz/qbe-go/ir"
    "github.com/ugurkorkmaz/qbe-go/opt"
)

func main() {
	// 1. Build IR using the Builder API
	b := builder.NewBuilder("add")
	b.Block("start")
	p1 := b.Param(ir.Kw, "a") // 32-bit integer
	p2 := b.Param(ir.Kw, "b")
	res := b.Add(ir.Kw, p1, p2)
	b.Ret(ir.Kw, res)
	f := b.Build()
    f.Exported = true

	// 2. Select Target (ARM64 Apple Silicon)
	target := &arm64.ARM64Target{Apple: true}

	// 3. Compile Pipeline
	analysis.SSA(f)            // Construct SSA
    opt.DCE(f)                 // Dead Code Elimination
	codegen.Spill(f, target)   // Handle register pressure
	target.ABI0(f)             // Lower to ABI (Argument passing)
	codegen.NewRegAllocator(f, target).Allocate()

	// 4. Emit Assembly
    // Pass global symbol names to resolve inter-module references
    globals := []string{"_add"}
	if err := target.Emit(f, globals); err != nil {
		log.Fatal(err)
	}
}
```

## Comparisons with C QBE

We strive for bit-perfect logical equivalence with the original QBE.

| Feature | Original QBE (C) | QBE-Go |
|---------|------------------|--------|
| **Language** | C99 | Go 1.20+ |
| **Input** | `.ssa` text file | Go Builder API (Text parser planned) |
| **Backend** | AMD64, ARM64, RISC-V | ARM64 (Apple Optimized) |
| **Dependencies** | None (libc) | None (Go stdlib) |
| **Safety** | Manual memory mgmt | Garbage Collected |

For a simple `add` function, QBE-Go calls generate clean assembly identical to hand-written code:

```asm
_add:
       stp x29, x30, [sp, #-16]!
       mov x29, sp
       add w0, w0, w1
       mov sp, x29
       ldp x29, x30, [sp], #16
       ret
```

## Roadmap

- [ ] **Text Parser**: A full parser for `.ssa` text files to allow standalone usage like the original tool.
- [ ] **WebAssembly (Wasm) Backend**: To run Sand/QBE-Go in the browser.
- [ ] **AMD64 (x86_64) Target**: Porting existing AMD64 logic from C.
- [ ] **Polishing**: More peephole optimizations (e.g., `madd` instruction fusion).

## Acknowledgements

Huge thanks to **Quentin Carbonneaux** for creating the original [QBE](https://c9x.me/compile/). His design proves that compilers don't have to be black magic or millions of lines of code.

This project is a tribute to that elegance.
