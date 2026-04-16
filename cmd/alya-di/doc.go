// Package main contains the alya-di generator.
//
// The generator is split into three stages:
//   - load.go reads source and type information from the package that declares
//     the graph.
//   - resolve.go turns the declarative graph into an ordered build plan.
//   - emit.go renders the final Go source for the generated Build function.
//
// The DSL and graph rules stay in Alya-specific code. The emitter uses a
// proven Go code generation library so the file layout, imports, and
// formatting stay predictable without hand-built string concatenation.
package main
