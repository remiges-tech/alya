package main

import (
	"strings"
	"testing"
)

func TestOutputFieldNamesUsePackageQualifierOnCollision(t *testing.T) {
	ctx, err := loadPackageContext(".", "./testdata/output_field_names")
	if err != nil {
		t.Fatalf("load package context: %v", err)
	}
	model, err := ctx.parseGraph("Graph")
	if err != nil {
		t.Fatalf("parse graph: %v", err)
	}
	resolved, err := resolveGraph(model)
	if err != nil {
		t.Fatalf("resolve graph: %v", err)
	}
	imports := ctx.collectImports(model)
	generated, err := emitBuildFile(model, resolved, imports)
	if err != nil {
		t.Fatalf("emit build file: %v", err)
	}
	content := string(generated)

	checks := []string{
		"FooHandler *foo.Handler",
		"BarHandler *bar.Handler",
		"app.FooHandler = ",
		"app.BarHandler = ",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Fatalf("generated file does not contain %q\n%s", check, content)
		}
	}
	if strings.Contains(content, "Handler2") {
		t.Fatalf("generated file still uses numeric suffix for colliding output names\n%s", content)
	}
}
