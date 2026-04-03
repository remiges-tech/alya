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

// TestOutputFieldNamesUsesCustomNameViaNamed verifies that di.Named(...)
// supplies the field name and that plain di.Type[T]() still gets an
// auto-generated name.
func TestOutputFieldNamesUsesCustomNameViaNamed(t *testing.T) {
	ctx, err := loadPackageContext(".", "./testdata/output_custom_names")
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

	// Custom names from di.Named must appear as struct fields and in assignments.
	for _, name := range []string{"PrimaryHandler", "SecondaryHandler"} {
		if !strings.Contains(content, "app."+name+" = ") {
			t.Fatalf("build should assign to app.%s\n%s", name, content)
		}
	}

	// The plain di.Type[T]() entry should still get an auto-generated name.
	if !strings.Contains(content, "app.DefaultSvc = ") {
		t.Fatalf("build should assign to app.DefaultSvc\n%s", content)
	}
}
