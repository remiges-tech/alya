// Package di declares a small, compile-time dependency graph description API.
//
// The package is intentionally tiny. It does not perform dependency resolution at
// runtime and it does not provide a service locator or container. Its only job
// is to let users describe a dependency graph in ordinary Go code so that the
// `alya-di` code generator can inspect that graph and emit plain bootstrap code.
//
// The model is constructor-based dependency injection:
//
//   - users write normal provider functions such as `NewDB`, `NewRepo`, or
//     `NewHandler`
//   - users register those provider functions with `di.Provide(...)`
//   - users describe graph inputs with `di.Inputs(...)`
//   - users describe desired outputs with `di.Outputs(...)`
//   - users register side-effect functions such as route registration with
//     `di.Invoke(...)`
//   - users add explicit interface bindings with `di.Bind[Interface, Concrete]()`
//
// The generated code performs the actual injection by calling those providers in
// dependency order. Because the generator works from typed constructor
// signatures, dependencies remain explicit and compile-time checked.
//
// This package is deliberately declarative. The exported functions return simple
// marker values so application code compiles, but the values themselves are not
// used at runtime. The `alya-di` generator reads the graph from source and type
// information instead.
package di
