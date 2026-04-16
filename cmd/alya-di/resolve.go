package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"strings"
)

// resolveGraph turns the declarative model into a concrete build plan.
//
// Resolution starts from requested outputs and invoke parameters, walks provider
// dependencies recursively, applies explicit interface bindings, and produces a
// topologically sorted provider list for code generation.
func resolveGraph(model graphModel) (resolvedGraph, error) {
	providerByType := make(map[string]providerRef, len(model.providers))
	for _, provider := range model.providers {
		if existing, ok := providerByType[provider.result.typeKey]; ok {
			return resolvedGraph{}, fmt.Errorf("ambiguous provider for %s: %s and %s", provider.result.exprString, existing.exprString, provider.exprString)
		}
		providerByType[provider.result.typeKey] = provider
	}

	inputByType := make(map[string]typeRef, len(model.inputs))
	for _, input := range model.inputs {
		if _, ok := inputByType[input.typeKey]; ok {
			return resolvedGraph{}, fmt.Errorf("duplicate input for %s", input.exprString)
		}
		inputByType[input.typeKey] = input
	}

	bindingByInterface := make(map[string]bindingRef, len(model.bindings))
	for _, binding := range model.bindings {
		if _, ok := bindingByInterface[binding.interfaceType.typeKey]; ok {
			return resolvedGraph{}, fmt.Errorf("duplicate binding for %s", binding.interfaceType.exprString)
		}
		bindingByInterface[binding.interfaceType.typeKey] = binding
	}

	baseFieldNameCounts := make(map[string]int, len(model.outputs))
	for _, output := range model.outputs {
		baseFieldNameCounts[baseFieldName(output.typeValue)]++
	}

	state := resolverState{
		providerByType:      providerByType,
		inputByType:         inputByType,
		bindingByInterface:  bindingByInterface,
		visiting:            map[string]bool{},
		resolved:            map[string]*providerNode{},
		providerOrder:       []*providerNode{},
		fieldNames:          map[string]int{},
		baseFieldNameCounts: baseFieldNameCounts,
	}

	resolved := resolvedGraph{}
	for _, output := range model.outputs {
		source, err := state.resolveDependency(output)
		if err != nil {
			return resolvedGraph{}, err
		}
		resolved.outputs = append(resolved.outputs, resolvedOutput{
			fieldName: state.makeFieldName(output),
			requested: output,
			source:    source,
		})
	}

	for _, invoke := range model.invokes {
		resolvedInvoke := resolvedInvoke{ref: invoke}
		for _, param := range invoke.params {
			source, err := state.resolveDependency(param)
			if err != nil {
				return resolvedGraph{}, fmt.Errorf("invoke %s: %w", invoke.exprString, err)
			}
			resolvedInvoke.dependencies = append(resolvedInvoke.dependencies, dependencyRef{requested: param, source: source})
		}
		resolved.invokes = append(resolved.invokes, resolvedInvoke)
	}

	resolved.providerOrder = state.providerOrder
	return resolved, nil
}

// resolverState holds the mutable state needed while recursively resolving one
// graph.
type resolverState struct {
	providerByType      map[string]providerRef
	inputByType         map[string]typeRef
	bindingByInterface  map[string]bindingRef
	visiting            map[string]bool
	resolved            map[string]*providerNode
	providerOrder       []*providerNode
	fieldNames          map[string]int
	baseFieldNameCounts map[string]int
}

// resolveDependency resolves one requested type to either an input or a concrete
// provider output. The returned typeRef is always the concrete source value that
// code generation should use.
func (r *resolverState) resolveDependency(requested typeRef) (typeRef, error) {
	if input, ok := r.inputByType[requested.typeKey]; ok {
		return input, nil
	}

	if binding, ok := r.bindingByInterface[requested.typeKey]; ok {
		if err := r.resolveProvider(binding.concreteType); err != nil {
			return typeRef{}, err
		}
		return binding.concreteType, nil
	}

	if _, ok := r.providerByType[requested.typeKey]; ok {
		if err := r.resolveProvider(requested); err != nil {
			return typeRef{}, err
		}
		return requested, nil
	}

	return typeRef{}, fmt.Errorf("no provider or input found for %s", requested.exprString)
}

// resolveProvider resolves and schedules the provider that produces target.
func (r *resolverState) resolveProvider(target typeRef) error {
	if _, ok := r.resolved[target.typeKey]; ok {
		return nil
	}
	if r.visiting[target.typeKey] {
		return fmt.Errorf("dependency cycle detected while resolving %s", target.exprString)
	}
	provider, ok := r.providerByType[target.typeKey]
	if !ok {
		return fmt.Errorf("no provider registered for %s", target.exprString)
	}

	r.visiting[target.typeKey] = true
	node := &providerNode{ref: provider}
	for _, param := range provider.params {
		source, err := r.resolveDependency(param)
		if err != nil {
			return fmt.Errorf("provider %s: %w", provider.exprString, err)
		}
		node.dependencies = append(node.dependencies, dependencyRef{requested: param, source: source})
	}
	delete(r.visiting, target.typeKey)
	r.resolved[target.typeKey] = node
	r.providerOrder = append(r.providerOrder, node)
	return nil
}

// makeFieldName returns the field name for one App output. If the output was
// declared with di.Named, the custom name is used directly. Otherwise a name
// is derived from the type, with a numeric suffix for collisions.
//
// No generator-side check for duplicate custom names is performed here. If two
// Outputs entries declare the same custom name, the Go compiler emits a clear
// duplicate field error when the generated code compiles. The generator only
// validates things the compiler cannot, such as cycles and missing providers.
//
// TODO: Consider adding generator-side duplicate name detection so users get
// the error during code generation instead of waiting for go build to fail.
func (r *resolverState) makeFieldName(output typeRef) string {
	if output.customName != "" {
		return output.customName
	}
	name := baseFieldName(output.typeValue)
	if r.baseFieldNameCounts[name] > 1 {
		if qualifier := outputFieldQualifier(output.exprString); qualifier != "" {
			name = qualifier + name
		}
	}
	count := r.fieldNames[name]
	r.fieldNames[name] = count + 1
	if count == 0 {
		return name
	}
	return fmt.Sprintf("%s%d", name, count+1)
}

func baseFieldName(t types.Type) string {
	name := baseTypeName(t)
	if name == "" {
		return "Value"
	}
	return name
}

func outputFieldQualifier(expr string) string {
	parsed, err := parser.ParseExpr(expr)
	if err != nil {
		return ""
	}
	return exportedQualifierName(typeQualifier(parsed))
}

func typeQualifier(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.ParenExpr:
		return typeQualifier(typed.X)
	case *ast.StarExpr:
		return typeQualifier(typed.X)
	case *ast.IndexExpr:
		return typeQualifier(typed.X)
	case *ast.IndexListExpr:
		return typeQualifier(typed.X)
	case *ast.SelectorExpr:
		if ident, ok := typed.X.(*ast.Ident); ok {
			return ident.Name
		}
		return typeQualifier(typed.X)
	}
	return ""
}

func exportedQualifierName(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.Split(name, "_")
	result := strings.Builder{}
	for _, part := range parts {
		if part == "" {
			continue
		}
		result.WriteString(upperFirst(part))
	}
	return result.String()
}
