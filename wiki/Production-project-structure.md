# Production project structure

This note defines the default directory structure for new production web services built with Alya.

Theme: keep the entrypoint small, keep app code under `internal/`, and add `pkg/` only when code must be imported outside the module.

## Recommended structure

```text
project/
  cmd/
    service/
      main.go

  internal/
    http/            handlers, routes, middleware
    service/         business logic
    repository/      repository interfaces and implementations
    db/              postgres setup, SQLC config, queries, generated code, migrations
    api/             request and response types
    model/           domain models
    validation/      validation setup

  pkg/               optional shared public code

  go.mod
  go.sum
  README.md
```

## What each directory is for

### `cmd/`

Use `cmd/` for application entrypoints.

For most services, one entrypoint is enough:

```text
cmd/service/main.go
```

Keep `main.go` focused on startup wiring:

- load configuration
- initialize database and other dependencies
- build services and handlers
- register routes
- start the server

### `internal/`

Put app-specific code under `internal/`.

This keeps the service structure explicit and prevents accidental reuse by other modules.

#### `internal/http/`

HTTP transport code:

- handlers
- routes
- middleware
- transport-specific helpers

Use:

- `restutils` for direct JSON REST APIs
- `wscutils` for Alya envelope-style APIs

#### `internal/service/`

Business logic and use cases.

Services should:

- receive typed dependencies through constructors
- call repositories
- stay independent of HTTP details

#### `internal/repository/`

Repository interfaces and implementations.

This layer:

- defines the operations needed by the service layer
- hides SQLC and database details from business logic

#### `internal/db/`

Database-related code and assets.

Typical contents:

```text
internal/db/
  pg.go
  sqlc.yaml
  queries/
  sqlc/
  migrations/
```

#### `internal/api/`

Request and response types used at the API boundary.

Use this for:

- create request structs
- update request structs
- response structs

#### `internal/model/`

Domain models used inside the application.

Use this when business entities should stay separate from API types and SQLC-generated types.

#### `internal/validation/`

Validation setup and validation helpers.

This is the place for:

- validator initialization
- custom validation rules
- transport-specific validation mapping

### `pkg/`

Use `pkg/` only for code that must be imported by other modules.

If code is only used by this service, keep it in `internal/`.

If you do not have shared public code yet, do not create `pkg/`.

## Flow

Keep the request flow simple:

```text
http -> service -> repository -> db
```

This means:

- handlers parse requests and send responses
- services implement business rules
- repositories handle persistence
- database code stays under `db/`

## Relation to the examples

The examples now show both layouts:

- `examples/rest-usersvc-sqlc-example` keeps the flatter example layout
- `examples/wsc-usersvc-sqlc-example` follows the `cmd/` and `internal/` layout in this note

Use the flatter layout when you want a compact teaching example.

Use the `cmd/` and `internal/` layout for production services.

## Default rules

Use these rules unless a service has a specific reason to differ:

- keep `main.go` small
- put app-specific code in `internal/`
- use `http/` for transport code
- use `service/` for business logic
- use `repository/` for persistence contracts and implementations
- keep SQLC assets under `db/`
- keep request and response types under `api/`
- add `model/` when domain types need to stay separate
- add `pkg/` only for public reusable code
