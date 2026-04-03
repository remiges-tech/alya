package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const diImportPath = "github.com/remiges-tech/alya/di"

// packageContext contains the syntax and type information needed to inspect one
// Go package that declares a DI graph.
type packageContext struct {
	pkg         *packages.Package
	fset        *token.FileSet
	varExprs    map[string]ast.Expr
	importNames map[string]string
}

// loadPackageContext loads the requested package with full syntax and type
// information, then indexes package-scope variables so graph declarations and
// reusable modules can be resolved by name.
func loadPackageContext(dir, pattern string) (*packageContext, error) {
	cfg := &packages.Config{
		Dir:  dir,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, fmt.Errorf("load package %q: %w", pattern, err)
	}
	// Do not fail fast on package errors here.
	//
	// A common first-run workflow is:
	//   1. write graph.go
	//   2. call the generated Build(...) from main.go
	//   3. run go generate
	//
	// Before generation, the package can still contain an undefined Build symbol.
	// packages.Load usually still gives us enough syntax and type information to
	// inspect the graph declaration, so the generator should continue when it can.
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("expected exactly one package for %q, got %d", pattern, len(pkgs))
	}

	ctx := &packageContext{
		pkg:         pkgs[0],
		fset:        pkgs[0].Fset,
		varExprs:    make(map[string]ast.Expr),
		importNames: make(map[string]string),
	}
	for _, file := range pkgs[0].Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok || len(valueSpec.Names) == 0 || len(valueSpec.Values) == 0 {
					continue
				}
				for i, name := range valueSpec.Names {
					valueIndex := i
					if valueIndex >= len(valueSpec.Values) {
						valueIndex = len(valueSpec.Values) - 1
					}
					ctx.varExprs[name.Name] = valueSpec.Values[valueIndex]
				}
			}
		}
		for _, imp := range file.Imports {
			path, err := stringLiteralValue(imp.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("parse import path %s: %w", imp.Path.Value, err)
			}
			name := filepath.Base(path)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			ctx.importNames[name] = path
		}
	}
	return ctx, nil
}

// parseGraph loads one graph or module variable and expands nested Includes.
func (ctx *packageContext) parseGraph(name string) (graphModel, error) {
	expr, ok := ctx.varExprs[name]
	if !ok {
		return graphModel{}, fmt.Errorf("graph or module %q was not found", name)
	}
	model := graphModel{packageName: ctx.pkg.Name, imports: map[string]importRef{}}
	if err := ctx.mergeGraphExpr(&model, expr, map[string]bool{}); err != nil {
		return graphModel{}, err
	}
	return model, nil
}

// mergeGraphExpr parses one di.New(...) or di.Module(...) call and merges its
// declarations into model.
func (ctx *packageContext) mergeGraphExpr(model *graphModel, expr ast.Expr, seen map[string]bool) error {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return fmt.Errorf("graph declaration must be a function call")
	}
	name, _, ok := calledDIFunction(call, ctx.pkg.TypesInfo)
	if !ok || (name != "New" && name != "Module") {
		return fmt.Errorf("graph declaration must use di.New(...) or di.Module(...)")
	}

	for _, arg := range call.Args {
		optionCall, ok := arg.(*ast.CallExpr)
		if !ok {
			return fmt.Errorf("graph option %s must be a function call", ctx.nodeString(arg))
		}
		optionName, typeArgs, ok := calledDIFunction(optionCall, ctx.pkg.TypesInfo)
		if !ok {
			return fmt.Errorf("unsupported graph option %s", ctx.nodeString(arg))
		}
		switch optionName {
		case "Inputs":
			refs, err := ctx.parseTypeTokens(optionCall.Args)
			if err != nil {
				return fmt.Errorf("parse Inputs: %w", err)
			}
			model.inputs = append(model.inputs, refs...)
			for _, arg := range optionCall.Args {
				ctx.addImportsFromNode(model, arg)
			}
		case "Outputs":
			refs, err := ctx.parseTypeTokens(optionCall.Args)
			if err != nil {
				return fmt.Errorf("parse Outputs: %w", err)
			}
			model.outputs = append(model.outputs, refs...)
			for _, arg := range optionCall.Args {
				ctx.addImportsFromNode(model, arg)
			}
		case "Provide":
			providers, err := ctx.parseProviders(optionCall.Args)
			if err != nil {
				return fmt.Errorf("parse Provide: %w", err)
			}
			model.providers = append(model.providers, providers...)
			for _, arg := range optionCall.Args {
				ctx.addImportsFromNode(model, arg)
			}
		case "Invoke":
			invokes, err := ctx.parseInvokes(optionCall.Args)
			if err != nil {
				return fmt.Errorf("parse Invoke: %w", err)
			}
			model.invokes = append(model.invokes, invokes...)
			for _, arg := range optionCall.Args {
				ctx.addImportsFromNode(model, arg)
			}
		case "Bind":
			binding, err := ctx.parseBinding(typeArgs)
			if err != nil {
				return fmt.Errorf("parse Bind: %w", err)
			}
			model.bindings = append(model.bindings, binding)
		case "Include":
			for _, includeExpr := range optionCall.Args {
				ident, ok := includeExpr.(*ast.Ident)
				if !ok {
					return fmt.Errorf("Include expects module identifiers, got %s", ctx.nodeString(includeExpr))
				}
				if seen[ident.Name] {
					return fmt.Errorf("module include cycle detected at %q", ident.Name)
				}
				moduleExpr, ok := ctx.varExprs[ident.Name]
				if !ok {
					return fmt.Errorf("included module %q was not found", ident.Name)
				}
				nextSeen := make(map[string]bool, len(seen)+1)
				for key, value := range seen {
					nextSeen[key] = value
				}
				nextSeen[ident.Name] = true
				if err := ctx.mergeGraphExpr(model, moduleExpr, nextSeen); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported graph option di.%s", optionName)
		}
	}
	return nil
}

// parseTypeTokens converts di.Type[T]() and di.Named(..., di.Type[T]()) calls
// into type references.
func (ctx *packageContext) parseTypeTokens(args []ast.Expr) ([]typeRef, error) {
	refs := make([]typeRef, 0, len(args))
	for _, arg := range args {
		call, ok := arg.(*ast.CallExpr)
		if !ok {
			return nil, fmt.Errorf("type token must be a function call, got %s", ctx.nodeString(arg))
		}
		name, typeArgs, ok := calledDIFunction(call, ctx.pkg.TypesInfo)
		if !ok {
			return nil, fmt.Errorf("expected di.Type[T]() or di.Named(...), got %s", ctx.nodeString(arg))
		}
		switch name {
		case "Type":
			if len(typeArgs) != 1 {
				return nil, fmt.Errorf("di.Type requires exactly one type argument")
			}
			refs = append(refs, ctx.makeTypeRef(typeArgs[0]))
		case "Named":
			if len(call.Args) != 2 {
				return nil, fmt.Errorf("di.Named requires exactly two arguments")
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return nil, fmt.Errorf("di.Named first argument must be a string literal")
			}
			customName, err := stringLiteralValue(lit.Value)
			if err != nil {
				return nil, fmt.Errorf("di.Named string literal: %w", err)
			}
			namedCall, ok := call.Args[1].(*ast.CallExpr)
			if !ok {
				return nil, fmt.Errorf("di.Named second argument must be a function call")
			}
			innerName, innerTypeArgs, ok := calledDIFunction(namedCall, ctx.pkg.TypesInfo)
			if !ok || innerName != "Type" {
				return nil, fmt.Errorf("di.Named second argument must be di.Type[T]()")
			}
			if len(innerTypeArgs) != 1 {
				return nil, fmt.Errorf("di.Named di.Type requires exactly one type argument")
			}
			ref := ctx.makeTypeRef(innerTypeArgs[0])
			ref.customName = customName
			refs = append(refs, ref)
		default:
			return nil, fmt.Errorf("expected di.Type[T]() or di.Named(...), got %s", ctx.nodeString(arg))
		}
	}
	return refs, nil
}

// parseProviders validates provider signatures and converts them into providerRef
// values the resolver can work with.
func (ctx *packageContext) parseProviders(args []ast.Expr) ([]providerRef, error) {
	providers := make([]providerRef, 0, len(args))
	for _, arg := range args {
		sig, ok := ctx.pkg.TypesInfo.TypeOf(arg).Underlying().(*types.Signature)
		if !ok {
			return nil, fmt.Errorf("provider %s is not a function", ctx.nodeString(arg))
		}
		if sig.Variadic() {
			return nil, fmt.Errorf("provider %s must not be variadic", ctx.nodeString(arg))
		}
		provider := providerRef{exprString: ctx.nodeString(arg)}
		provider.params = signatureParams(sig, ctx)
		resultType, hasError, hasCleanup, err := parseProviderResults(sig)
		if err != nil {
			return nil, fmt.Errorf("provider %s: %w", provider.exprString, err)
		}
		provider.result = ctx.makeTypeRefFromType(resultType)
		provider.hasError = hasError
		provider.hasCleanup = hasCleanup
		providers = append(providers, provider)
	}
	return providers, nil
}

// parseInvokes validates invoke function signatures.
func (ctx *packageContext) parseInvokes(args []ast.Expr) ([]invokeRef, error) {
	invokes := make([]invokeRef, 0, len(args))
	for _, arg := range args {
		sig, ok := ctx.pkg.TypesInfo.TypeOf(arg).Underlying().(*types.Signature)
		if !ok {
			return nil, fmt.Errorf("invoke %s is not a function", ctx.nodeString(arg))
		}
		if sig.Variadic() {
			return nil, fmt.Errorf("invoke %s must not be variadic", ctx.nodeString(arg))
		}
		returnsErr, err := parseInvokeResults(sig)
		if err != nil {
			return nil, fmt.Errorf("invoke %s: %w", ctx.nodeString(arg), err)
		}
		invokes = append(invokes, invokeRef{
			exprString: ctx.nodeString(arg),
			params:     signatureParams(sig, ctx),
			returnsErr: returnsErr,
		})
	}
	return invokes, nil
}

// parseBinding extracts the two type arguments supplied to di.Bind[Interface,
// Concrete]().
func (ctx *packageContext) parseBinding(typeArgs []ast.Expr) (bindingRef, error) {
	if len(typeArgs) != 2 {
		return bindingRef{}, fmt.Errorf("di.Bind requires exactly two type arguments")
	}
	binding := bindingRef{
		interfaceType: ctx.makeTypeRef(typeArgs[0]),
		concreteType:  ctx.makeTypeRef(typeArgs[1]),
	}
	iface, ok := binding.interfaceType.typeValue.Underlying().(*types.Interface)
	if !ok {
		return bindingRef{}, fmt.Errorf("first Bind type must be an interface, got %s", binding.interfaceType.exprString)
	}
	if !types.AssignableTo(binding.concreteType.typeValue, iface) {
		return bindingRef{}, fmt.Errorf("%s does not implement %s", binding.concreteType.exprString, binding.interfaceType.exprString)
	}
	return binding, nil
}

// makeTypeRef converts one AST type expression into a stable type reference.
func (ctx *packageContext) makeTypeRef(expr ast.Expr) typeRef {
	t := ctx.pkg.TypesInfo.TypeOf(expr)
	return typeRef{
		typeKey:    typeKey(t),
		typeValue:  t,
		exprString: ctx.nodeString(expr),
	}
}

// makeTypeRefFromType builds a typeRef from a resolved types.Type. The source
// expression string is derived from the package's import names so the generated
// code can reuse package qualifiers consistently.
func (ctx *packageContext) makeTypeRefFromType(t types.Type) typeRef {
	return typeRef{
		typeKey:    typeKey(t),
		typeValue:  t,
		exprString: ctx.typeString(t),
	}
}

// addImportsFromNode records imported packages referenced by node. The AST nodes
// come from the loaded package syntax, so package-qualified selector expressions
// can be resolved through types.Info.
func (ctx *packageContext) addImportsFromNode(model *graphModel, node ast.Node) {
	ast.Inspect(node, func(current ast.Node) bool {
		selector, ok := current.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		pkgObject, ok := ctx.pkg.TypesInfo.Uses[ident].(*types.PkgName)
		if !ok {
			return true
		}
		alias := ident.Name
		path := pkgObject.Imported().Path()
		if path == ctx.pkg.Types.Path() || path == diImportPath {
			return true
		}
		model.imports[alias] = importRef{Alias: alias, Path: path}
		return true
	})
}

// nodeString pretty-prints one syntax node exactly once so the generated code
// can reuse provider expressions and type expressions as written by the user.
func (ctx *packageContext) nodeString(node ast.Node) string {
	var buf bytes.Buffer
	_ = format.Node(&buf, ctx.fset, node)
	return buf.String()
}

// typeString renders a Go type with package qualifiers chosen from the source
// package's imports.
func (ctx *packageContext) typeString(t types.Type) string {
	qualifier := func(pkg *types.Package) string {
		if pkg == nil || pkg.Path() == ctx.pkg.Types.Path() {
			return ""
		}
		for name, path := range ctx.importNames {
			if path == pkg.Path() {
				return name
			}
		}
		return pkg.Name()
	}
	return types.TypeString(t, qualifier)
}

// signatureParams converts a function signature's parameters into typeRefs.
func signatureParams(sig *types.Signature, ctx *packageContext) []typeRef {
	params := make([]typeRef, 0, sig.Params().Len())
	for i := 0; i < sig.Params().Len(); i++ {
		params = append(params, ctx.makeTypeRefFromType(sig.Params().At(i).Type()))
	}
	return params
}

// parseProviderResults validates the supported provider return shapes.
func parseProviderResults(sig *types.Signature) (types.Type, bool, bool, error) {
	results := sig.Results()
	switch results.Len() {
	case 1:
		return results.At(0).Type(), false, false, nil
	case 2:
		if !isErrorType(results.At(1).Type()) {
			return nil, false, false, fmt.Errorf("supported two-result provider shape is (T, error)")
		}
		return results.At(0).Type(), true, false, nil
	case 3:
		if !isCleanupFunc(results.At(1).Type()) || !isErrorType(results.At(2).Type()) {
			return nil, false, false, fmt.Errorf("supported three-result provider shape is (T, func(), error)")
		}
		return results.At(0).Type(), true, true, nil
	default:
		return nil, false, false, fmt.Errorf("provider must return T, (T, error), or (T, func(), error)")
	}
}

// parseInvokeResults validates the supported invoke return shapes.
func parseInvokeResults(sig *types.Signature) (bool, error) {
	results := sig.Results()
	switch results.Len() {
	case 0:
		return false, nil
	case 1:
		if !isErrorType(results.At(0).Type()) {
			return false, fmt.Errorf("supported invoke result shape is error")
		}
		return true, nil
	default:
		return false, fmt.Errorf("invoke must return nothing or error")
	}
}

// calledDIFunction reports the di helper used by call. It also returns any type
// arguments that were supplied to that helper.
func calledDIFunction(call *ast.CallExpr, info *types.Info) (string, []ast.Expr, bool) {
	base := call.Fun
	var typeArgs []ast.Expr
	switch fun := call.Fun.(type) {
	case *ast.IndexExpr:
		base = fun.X
		typeArgs = []ast.Expr{fun.Index}
	case *ast.IndexListExpr:
		base = fun.X
		typeArgs = append(typeArgs, fun.Indices...)
	}
	selector, ok := base.(*ast.SelectorExpr)
	if !ok {
		return "", nil, false
	}
	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", nil, false
	}
	pkgObject, ok := info.Uses[pkgIdent].(*types.PkgName)
	if !ok || pkgObject.Imported().Path() != diImportPath {
		return "", nil, false
	}
	return selector.Sel.Name, typeArgs, true
}

// typeKey produces a stable identifier for a Go type using package paths instead
// of local import aliases.
func typeKey(t types.Type) string {
	qualifier := func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Path()
	}
	return types.TypeString(t, qualifier)
}

// stringLiteralValue removes the quotes from one import path literal.
func stringLiteralValue(raw string) (string, error) {
	if len(raw) < 2 {
		return "", fmt.Errorf("invalid quoted string %q", raw)
	}
	return strings.Trim(raw, "\""), nil
}

// isErrorType reports whether t is the predeclared error interface.
func isErrorType(t types.Type) bool {
	return typeKey(t) == "error"
}

// isCleanupFunc reports whether t is a zero-arg, zero-result function type.
func isCleanupFunc(t types.Type) bool {
	sig, ok := t.Underlying().(*types.Signature)
	if !ok {
		return false
	}
	return !sig.Variadic() && sig.Params().Len() == 0 && sig.Results().Len() == 0
}

// collectImports returns the external packages referenced by the graph. The
// imports were recorded while parsing the original AST, so package aliases match
// the user's source files.
func (ctx *packageContext) collectImports(model graphModel) []importRef {
	result := make([]importRef, 0, len(model.imports))
	for _, ref := range model.imports {
		result = append(result, ref)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Alias == result[j].Alias {
			return result[i].Path < result[j].Path
		}
		return result[i].Alias < result[j].Alias
	})
	return result
}
