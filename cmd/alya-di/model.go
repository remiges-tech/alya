package main

import "go/types"

// graphModel is the fully parsed graph declaration used by the generator.
//
// The model keeps both the human-facing source strings we want to reuse in the
// generated file and the resolved Go types we need for dependency resolution.
type graphModel struct {
	packageName string
	inputs      []typeRef
	outputs     []typeRef
	providers   []providerRef
	invokes     []invokeRef
	bindings    []bindingRef
	imports     map[string]importRef
}

// importRef describes one import required by the generated file.
type importRef struct {
	Alias string
	Path  string
}

// typeRef captures a type token from di.Type[T]().
//
// exprString preserves how the type was written in the source package so the
// generated file can reuse the same package qualifiers.
type typeRef struct {
	typeKey    string
	typeValue  types.Type
	exprString string
}

// providerRef describes one provider function registered through di.Provide.
type providerRef struct {
	exprString string
	params     []typeRef
	result     typeRef
	hasError   bool
	hasCleanup bool
}

// invokeRef describes one function registered through di.Invoke.
type invokeRef struct {
	exprString string
	params     []typeRef
	returnsErr bool
}

// bindingRef maps an interface dependency to a concrete provider result type.
type bindingRef struct {
	interfaceType typeRef
	concreteType  typeRef
}

// resolvedGraph is the dependency graph after all requirements have been
// resolved to concrete providers.
type resolvedGraph struct {
	providerOrder []*providerNode
	invokes       []resolvedInvoke
	outputs       []resolvedOutput
}

// providerNode represents a provider chosen for a specific concrete type.
type providerNode struct {
	ref          providerRef
	varName      string
	cleanupVar   string
	dependencies []dependencyRef
}

// dependencyRef records which concrete provider satisfies one constructor
// parameter. requested is the parameter type from the constructor. source is the
// concrete type that will be passed in.
type dependencyRef struct {
	requested typeRef
	source    typeRef
}

// resolvedInvoke stores the resolved dependency list for an invoke function.
type resolvedInvoke struct {
	ref          invokeRef
	dependencies []dependencyRef
}

// resolvedOutput stores the resolved provider or input used for a requested
// output type and the field name to expose in the generated App struct.
type resolvedOutput struct {
	fieldName string
	requested typeRef
	source    typeRef
}
