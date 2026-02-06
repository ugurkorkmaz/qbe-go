# Changelog

All notable changes to this project will be documented in this file.

## [v0.1.0] - 2026-02-07

### Added

- **Core Infrastructure**:
  - Implemented complete QBE Intermediate Representation (IR) in Go (`ir` package).
  - Added SSA (Static Single Assignment) construction and destruction analysis.
  - Implemented Control Flow Graph (CFG) and Dominator Tree analysis.

- **ARM64 Backend**:
  - Full ARM64 code generation support (`arch/arm64`).
  - ABI compliant function calls (AAPCS64), including HFA support.
  - Efficient stack frame management (prologue/epilogue).
  - Support for `fmov`, `fcvt`, and other floating-point operations.

- **Code Generation**:
  - **Register Allocation**: Implemented Linear Scan allocator with Copy Coalescing (`codegen/rega.go`).
  - **Spilling**: Automatic spill code generation for high register pressure.
  - **Data Sections**: Support for emitting global data (strings, constants).

- **Optimizations**:
  - **DCE**: Iterative Dead Code Elimination (`opt/dce.go`).
  - **GVN**: Scoped Global Value Numbering (`opt/gvn.go`).

- **Developer Experience**:
  - **Builder API**: Fluent, type-safe Go API for constructing IR (`builder` package).
  - **Testing**: Integration test suite with C interoperability (`test/`).
  - **Documentation**: Comprehensive README with architecture overview and examples.

### Fixed

- Resolved issues with stack alignment and variadic function calls (`printf`).
- Fixed symbol resolution for global data sections.

### Architecture

- Pure Go implementation with zero external dependencies.
- Modular design allowing easy addition of new targets (e.g., AMD64, Wasm).
