# wsc-usersvc-sqlc-example directory guide

This directory contains a WSC-style Alya service with two resources:

- `users`
- `orders`

It uses Alya envelope requests and responses, a service layer behind repository interfaces, and two repository adapters:

- SQLC
- GORM

Theme: the HTTP and service layers stay the same. Only the repository backend changes at startup.

## Top-level layout

```text
examples/wsc-usersvc-sqlc-example/
  cmd/
    service/
      main.go

  internal/
    api/
    db/
    http/
    repository/
    service/
    validation/

  config.json      configuration file (JSON format)
  docker-compose.yaml
  README.md
  STRUCTURE.md
```

## Directory roles

### `cmd/service/`

Service startup lives here.

File:
- `cmd/service/main.go`

This file:
- creates the logger
- loads startup config with Alya's `config` package
- selects the startup config source from `CONFIG_SOURCE`
- selects the backend from `ALYA_REPOSITORY_BACKEND`
- wires repositories, services, validators, and handlers
- starts Gin on the configured port

Startup config source values:
- empty or `file` -> load `config.json`
- `env` -> load environment variables with prefix `ALYA_WSC_USERSVC`

Backend values:
- empty or `sqlc` -> SQLC repository
- `gorm` -> GORM repository

This is the startup wiring point.

### `internal/api/`

Transport types live here.

Files:
- `internal/api/user_types.go`
- `internal/api/order_types.go`

These types define request and response shapes for handlers.
They do not depend on SQLC or GORM.

### `internal/http/`

Gin transport code lives here.

Files:
- `internal/http/routes.go`
- `internal/http/user_handler.go`
- `internal/http/order_handler.go`
- `internal/http/helpers.go`
- `internal/http/constants.go`

This package:
- binds Alya `{"data": ...}` requests
- validates input
- calls the service layer
- sends Alya envelope responses with `wscutils`

Routes are registered in `internal/http/routes.go`.

### `internal/service/`

Business rules live here.

Files:
- `internal/service/user_service.go`
- `internal/service/order_service.go`

This layer depends on repository interfaces, not on SQLC or GORM types.
That keeps the use cases independent from the persistence library.

Examples of rules in this layer:
- reject duplicate usernames
- reject duplicate order numbers
- require a valid user before creating an order
- map repository errors to application errors

### `internal/repository/`

Persistence contracts and adapters live here.

Files:
- `internal/repository/user_repository.go`
- `internal/repository/order_repository.go`
- `internal/repository/sqlc_repository.go`
- `internal/repository/gorm_repository.go`

This package contains:

1. repository interfaces and repository-level types
2. concrete backend implementations

Interfaces:
- `UserRepository`
- `OrderRepository`

Implementations:
- `SQLCRepository`
- `GORMRepository`

Both implementations return the same repository types and errors.
That keeps the service layer backend-neutral.

### `internal/db/`

Database setup and assets live here.

Contents:
- `internal/db/pg.go` -> SQLC database provider
- `internal/db/gorm.go` -> GORM database provider
- `internal/db/migrations/` -> tern migrations
- `internal/db/queries/` -> SQL files for SQLC
- `internal/db/sqlc.yaml` -> SQLC configuration
- `internal/db/sqlc/` -> generated SQLC code

This package opens database connections and holds database assets.
It does not contain business rules.

Important points:
- SQLC and GORM use the same PostgreSQL schema
- schema creation still comes from `tern migrate`
- GORM does not run auto-migrations in this example

### `internal/validation/`

Validation setup lives here.

File:
- `internal/validation/validator.go`

This package builds the `wscutils.Validator` instances used by handlers.
It maps validation failures to Alya message IDs and error codes.

## Request path

```text
HTTP request
  -> internal/http
  -> internal/service
  -> internal/repository
  -> internal/db
  -> PostgreSQL
```

Example for `POST /users`:

1. `internal/http/user_handler.go` binds and validates the request.
2. `internal/service/user_service.go` checks whether the username already exists.
3. `internal/repository/sqlc_repository.go` or `internal/repository/gorm_repository.go` queries the database.
4. The chosen repository creates the record.
5. The handler sends a WSC response.

## Why the split exists

Each directory has one job:

- `http` handles transport
- `service` handles use cases
- `repository` defines persistence contracts and adapters
- `db` holds database-specific setup and assets
- `api` holds request and response types
- `validation` holds validator setup

This makes one point clear in code:

- SQLC and GORM are implementation details behind the repository layer

## Backend switch

The backend switch happens only in:

- `cmd/service/main.go`

Run with SQLC:

```bash
cd examples/wsc-usersvc-sqlc-example
go run ./cmd/service
```

Expected result:
- the service starts on `:8084`

Run with GORM:

```bash
cd examples/wsc-usersvc-sqlc-example
ALYA_REPOSITORY_BACKEND=gorm go run ./cmd/service
```

Expected result:
- the service starts on `:8084`

These runs keep the same:
- routes
- handlers
- service logic
- request and response format
- database schema

Only the repository implementation changes.

## Files to read first

If you are new to this example, read in this order:

1. `README.md`
2. `cmd/service/main.go`
3. `internal/http/routes.go`
4. `internal/service/user_service.go`
5. `internal/repository/user_repository.go`
6. `internal/repository/sqlc_repository.go`
7. `internal/repository/gorm_repository.go`
8. `internal/db/pg.go`
9. `internal/db/gorm.go`

## Setup summary

Start PostgreSQL:

```bash
cd examples/wsc-usersvc-sqlc-example
docker compose up -d
```

Verify:

```bash
docker compose ps
```

Run migrations:

```bash
cd examples/wsc-usersvc-sqlc-example/internal/db/migrations
tern migrate
```

Expected result:
- the `users` and `orders` tables exist in PostgreSQL

Then start the service with either backend.

For full request examples, see:
- `README.md`
