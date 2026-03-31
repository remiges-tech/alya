# Alya compile-time DI

The `di` package and `alya-di` generator provide optional compile-time startup wiring for Alya services.

This feature is intentionally narrow:

- it uses normal constructor functions
- it generates plain Go bootstrap code
- it does not add a runtime container
- it does not add service-locator style dependency lookup

If your startup code is still short and readable, keep manual wiring. The generator helps when startup grows repetitive across services or modules.

## Mental model

Write constructors like this:

```go
func NewValidator() *restutils.Validator
func NewRepo(provider *pg.Provider) *repository.SQLCRepository
func NewUserService(repo repository.UserRepository) *app.UserService
func NewUserHandler(svc *app.UserService, validator *restutils.Validator) *transport.UserHandler
```

Then declare a graph:

```go
var Graph = di.New(
    di.Inputs(
        di.Type[*gin.Engine](),
        di.Type[AppConfig](),
    ),
    di.Provide(
        newPGConfig,
        newPGProvider,
        newSQLCRepository,
        newValidator,
        service.NewService,
        app.NewUserService,
        transport.NewUserHandler,
    ),
    di.Bind[repository.UserRepository, *repository.SQLCRepository](),
    di.Invoke(transport.RegisterRoutes),
    di.Outputs(di.Type[*transport.UserHandler]()),
)
```

Run the generator:

```bash
go run ./cmd/alya-di gen -graph Graph -out zz_generated_di.go ./examples/rest-usersvc-sqlc-example
```

The generator inspects provider signatures, resolves dependencies by type, and emits a `Build(...)` function.

## Terms

### Provider

A provider is a function registered with `di.Provide(...)`.

The current implementation supports only these provider signatures:

```go
func(...) T
func(...) (T, error)
func(...) (T, func(), error)
```

The first result is the provided value. The optional cleanup function is appended to the generated cleanup stack.

### Input

An input is a value supplied by the caller of the generated `Build(...)` function.

Declare inputs with `di.Inputs(...)`:

```go
di.Inputs(
    di.Type[*gin.Engine](),
    di.Type[AppConfig](),
)
```

That causes the generated build function to accept those values as parameters.

### Output

An output is a value you want the generated `Build(...)` function to return in the generated `App` struct.

Declare outputs with `di.Outputs(...)`:

```go
di.Outputs(
    di.Type[*transport.UserHandler](),
    di.Type[*transport.OrderHandler](),
)
```

### Invoke

An invoke function is called after the graph has been built.

Use it for side-effect setup such as route registration:

```go
di.Invoke(transport.RegisterRoutes)
```

The current implementation supports only:

```go
func(...)
func(...) error
```

### Bind

Bindings connect an interface dependency to a concrete provider result.

```go
di.Bind[repository.UserRepository, *repository.SQLCRepository]()
```

Bindings are explicit on purpose. The generator does not silently guess which concrete type should satisfy an interface.

### Module

Modules are reusable groups of providers, bindings, invokes, inputs, or outputs.

```go
var InfraModule = di.Module(
    di.Provide(newPGConfig, newPGProvider, newSQLCRepository),
)

var Graph = di.New(
    di.Inputs(di.Type[AppConfig]()),
    di.Include(InfraModule),
)
```

Modules are flattened at generation time. They have no runtime meaning.

## Why this is still dependency injection

This feature still uses dependency injection because components receive dependencies from the outside instead of constructing or discovering them internally.

This is DI:

```go
repo := NewRepo(provider)
svc := NewUserService(repo)
handler := NewUserHandler(svc, validator)
```

The generator only automates that constructor chain. It does not change the programming model.

## What the generator does

Given a graph declaration, the generator:

1. loads the package syntax and type information
2. reads `di.New(...)` and any included `di.Module(...)` declarations
3. inspects provider signatures
4. resolves dependencies by type
5. applies explicit interface bindings
6. validates missing dependencies, duplicate providers, and cycles
7. emits plain Go code with a `Build(...)` function and an `App` output struct

## What it does not do

It does not:

- perform runtime lookup
- use reflection to build objects at runtime
- inject a container into handlers or services
- add hidden dependencies
- replace normal constructors

## Current constraints

The initial implementation is intentionally small.

- Providers must be plain function values.
- Variadic providers are not supported.
- Invoke functions must return nothing or `error`.
- Interface bindings must be explicit.
- Resolution is type-based. If two providers return the same concrete type, generation fails.

## Generated cleanup

Providers with the signature `(T, func(), error)` can contribute cleanup steps.

The generated build function:

- stores successful cleanup functions in a stack
- runs them in reverse order on shutdown
- also runs them if a later provider or invoke step fails

## Example

See:

- `examples/rest-usersvc-sqlc-example/graph.go`
- `examples/rest-usersvc-sqlc-example/zz_generated_di.go`

That example shows:

- config and router as inputs
- provider wrappers for DB setup and repository construction
- explicit interface bindings
- route registration through `Invoke`
- generated startup wiring used from `main.go`
