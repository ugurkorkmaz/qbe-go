package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/arch/arm64"
	"github.com/ugurkorkmaz/qbe-go/codegen"
	"github.com/ugurkorkmaz/qbe-go/opt"
	"github.com/ugurkorkmaz/qbe-go/parser"
)

func main() {
	outPath := flag.String("o", "", "Output assembly file")
	targetName := flag.String("t", "arm64", "Target architecture (arm64)")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Printf("Usage: qbe-go [-o output.s] [-t target] <input.ssa>\n")
		os.Exit(1)
	}

	inputPath := flag.Arg(0)
	input, err := os.ReadFile(inputPath)
	if err != nil {
		log.Fatalf("Failed to read input: %v", err)
	}

	// 1. Parse SSA
	p := parser.NewParser(string(input))
	funcs := p.Parse()

	if len(funcs) == 0 {
		log.Fatalf("No functions found in input")
	}

	// 2. Setup Target
	var target *arm64.ARM64Target
	if *targetName == "arm64" {
		target = &arm64.ARM64Target{Apple: false}
	} else if *targetName == "arm64_apple" {
		target = &arm64.ARM64Target{Apple: true}
	} else {
		log.Fatalf("Unsupported target: %s", *targetName)
	}

	// 3. Setup Output
	var out io.Writer = os.Stdout
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer f.Close()
		out = f
	}
	target.Out = out

	// 4. Compile Pipeline
	globals := p.GetGlobals()
	for _, f := range funcs {
		analysis.SSA(f)
		target.Simplify(f)
		opt.DCE(f)
		opt.PhiElim(f)
		codegen.Spill(f, target)
		target.ABI0(f)
		codegen.NewRegAllocator(f, target).Allocate()

		if err := target.Emit(f, globals); err != nil {
			log.Fatalf("Emit failed for %s: %v", f.Name, err)
		}
	}
}
