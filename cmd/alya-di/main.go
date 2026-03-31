package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// main implements a small compile-time DI code generator.
//
// The only supported command today is:
//
//	alya-di gen -graph Graph -out zz_generated_di.go [package-pattern]
func main() {
	if len(os.Args) < 2 {
		fatalf("usage: alya-di gen -graph Graph -out zz_generated_di.go [package-pattern]")
	}

	switch os.Args[1] {
	case "gen":
		if err := runGen(os.Args[2:]); err != nil {
			fatalf("%v", err)
		}
	default:
		fatalf("unknown command %q", os.Args[1])
	}
}

// runGen loads the graph declaration, resolves its dependency graph, and writes
// the generated bootstrap file.
func runGen(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	graphName := fs.String("graph", "Graph", "package-level graph variable name")
	outPath := fs.String("out", "", "output Go file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *outPath == "" {
		return fmt.Errorf("-out is required")
	}

	pattern := "."
	if fs.NArg() > 0 {
		pattern = fs.Arg(0)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	ctx, err := loadPackageContext(dir, pattern)
	if err != nil {
		return err
	}
	model, err := ctx.parseGraph(*graphName)
	if err != nil {
		return err
	}
	resolved, err := resolveGraph(model)
	if err != nil {
		return err
	}
	imports := ctx.collectImports(model)
	generated, err := emitBuildFile(model, resolved, imports)
	if err != nil {
		return err
	}

	absOut := *outPath
	if !filepath.IsAbs(absOut) {
		absOut = filepath.Join(dir, *outPath)
	}
	if err := os.WriteFile(absOut, generated, 0o644); err != nil {
		return fmt.Errorf("write generated file %s: %w", absOut, err)
	}
	return nil
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
