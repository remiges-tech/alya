package di

// Graph is the root declaration for one generated bootstrap graph.
//
// A graph is a compile-time description only. It is meant to be assigned to a
// package-level variable, for example:
//
//	var Graph = di.New(
//		di.Inputs(di.Type[*gin.Engine](), di.Type[AppConfig]()),
//		di.Provide(NewLogger, NewRepo, NewService, NewHandler),
//		di.Outputs(di.Type[*Handler]()),
//	)
//
// The graph value itself is intentionally empty. The generator reads the source
// expression that produced it and performs all analysis from there.
type Graph struct{}

// ModuleSpec groups graph options that are shared across multiple graphs or used
// to keep one graph declaration readable. A module has no runtime meaning.
type ModuleSpec struct{}

// Option marks a value that can appear inside di.New(...) or di.Module(...).
//
// The concrete option values intentionally hold no data at runtime. They only
// exist so application packages compile cleanly while the generator reads the
// original source expressions.
type Option interface {
	isOption()
}

// TypeToken is a typed marker used by Inputs and Outputs. It lets callers refer
// to a Go type without passing a runtime value.
//
// Example:
//
//	di.Type[*gin.Engine]()
//	di.Type[AppConfig]()
//
// The generator reads the type argument and uses it as part of the dependency
// graph.
type TypeToken interface {
	isTypeToken()
}

// token is the concrete implementation behind TypeToken.
type token[T any] struct{}

func (token[T]) isTypeToken() {}

// Type returns a compile-time type marker for T.
//
// The marker is never inspected at runtime. The generator reads the type
// argument from source and type information.
func Type[T any]() TypeToken {
	return token[T]{}
}

// option is the concrete implementation behind Option.
//
// It carries no fields because the runtime values are not used. The graph is
// reconstructed by the generator from source, not from these instances.
type option struct{}

func (option) isOption() {}

// New declares a dependency graph.
//
// The returned Graph is a marker value. The important part is the source code of
// the options passed to this function.
func New(opts ...Option) Graph {
	return Graph{}
}

// Module declares a reusable group of graph options.
//
// Modules are flattened into the graph during code generation.
func Module(opts ...Option) ModuleSpec {
	return ModuleSpec{}
}

// Inputs declares values that must be supplied by the generated build function.
//
// Example:
//
//	di.Inputs(di.Type[*gin.Engine](), di.Type[AppConfig]())
//
// This causes the generator to emit a build function like:
//
//	func Build(engine *gin.Engine, cfg AppConfig) ...
func Inputs(types ...TypeToken) Option {
	return option{}
}

// Outputs declares values that the generated build function should return inside
// the generated App struct.
func Outputs(types ...TypeToken) Option {
	return option{}
}

// Provide registers provider functions with the graph.
//
// A provider function creates exactly one primary value and may optionally also
// return an error and a cleanup function. The initial implementation supports
// only these signatures:
//
//	func(...) T
//	func(...) (T, error)
//	func(...) (T, func(), error)
//
// Provider selection is based on types, not function names.
func Provide(functions ...any) Option {
	return option{}
}

// Invoke registers functions that should be called after dependencies are
// constructed.
//
// Invokes are useful for side-effect setup such as route registration or worker
// startup. The initial implementation supports only these signatures:
//
//	func(...)
//	func(...) error
func Invoke(functions ...any) Option {
	return option{}
}

// Include pulls reusable modules into a graph or another module.
func Include(modules ...ModuleSpec) Option {
	return option{}
}

// Bind declares that Concrete should be used when something depends on
// Interface.
//
// Bindings are explicit on purpose. The generator does not silently guess which
// concrete type should satisfy an interface dependency.
func Bind[Interface any, Concrete any]() Option {
	return option{}
}
