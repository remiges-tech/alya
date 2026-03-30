# Users and orders SQLC examples

This note explains the two SQLC examples added to Alya and what each example demonstrates.

Theme: the transport contract changes between the examples, but the same service layers stay in place.

## Examples

The examples are:

- `examples/rest-usersvc-sqlc-example`
- `examples/wsc-usersvc-sqlc-example`

Both examples implement the same resources:

- `users`
- `orders`

Both examples use:

- PostgreSQL
- SQLC
- repository interfaces
- app services
- typed constructor injection
- local database assets inside the example directory

## What is shared conceptually

Both examples use the same request flow:

```text
client request
  -> transport handler
  -> app service
  -> repository
  -> SQLC queries
  -> PostgreSQL
  -> response mapping
  -> client response
```

The directory layout is now slightly different:

- the REST example keeps the flatter example layout
- the WSC example follows the `cmd/` and `internal/` production layout

## The REST example

Path:

- `examples/rest-usersvc-sqlc-example`

### What it demonstrates

- modern REST-style request bodies
- plain JSON resource responses
- problem-style error responses
- typed dependency injection
- self-contained SQLC example layout

### Request and response style

Requests use direct JSON.

Example:

```json
{
  "name": "Sachin",
  "email": "sachin@example.com",
  "username": "sachin"
}
```

Responses return resources directly.

Example:

```json
{
  "id": 1,
  "name": "Sachin",
  "email": "sachin@example.com",
  "username": "sachin"
}
```

### Package focus

This example is the reference for:

- `restutils`
- HTTP-native transport style
- resource-oriented routes with direct JSON bodies

## The WSC example

Path:

- `examples/wsc-usersvc-sqlc-example`

### What it demonstrates

- Alya envelope-style request and response bodies
- additive `wscutils` transport helpers
- additive `wscutils.Validator`
- typed dependency injection
- minimal LogHarbour integration
- `cmd/` and `internal/` example layout

### Request and response style

Requests use Alya's `data` envelope.

Example:

```json
{
  "data": {
    "name": "Sachin",
    "email": "sachin@example.com",
    "username": "sachin"
  }
}
```

Responses use Alya's `status`, `data`, and `messages` envelope.

Example:

```json
{
  "status": "success",
  "data": {
    "id": 1,
    "name": "Sachin",
    "email": "sachin@example.com",
    "username": "sachin"
  },
  "messages": []
}
```

### Package focus

This example is the reference for:

- `wscutils` additive API
- Alya envelope-style transport
- JSON-tag-based validation with `msgid` and `errcode`
- typed logger injection into handlers

## Why both examples exist

Alya has two real requirements:

1. support existing Alya envelope-style services
2. support newer HTTP-native services

One example cannot show both clearly without mixing the transport styles.

Keeping two examples avoids that confusion.

## Why each example keeps its own database assets

Each example keeps its own:

- migrations
- SQL files
- `sqlc.yaml`
- generated SQLC files
- provider

The location differs by example:

- REST example: `pg/`
- WSC example: `internal/db/`

Reason:

- each example is self-contained
- setup instructions stay local to the example
- the example can be read without crossing into another example

## Why repository interfaces are still used even with SQLC

Both examples keep repository interfaces like:

- `UserRepository`
- `OrderRepository`

Reason:

- app services should not depend on SQLC query types
- repository interfaces express application-level operations
- SQLC stays behind the repository layer

The SQLC repository implementation depends on `sqlc.Querier`, not directly on `*sqlc.Queries`.

This keeps the SQLC detail inside the repository implementation.

## Validation setup in the WSC example

The WSC example includes an `internal/validation/` package.

Path:

- `examples/wsc-usersvc-sqlc-example/internal/validation/validator.go`

This package builds additive `wscutils.Validator` instances for:

- users
- orders

It maps validation tags like:

- `required`
- `email`
- `min`
- `max`
- `oneof`
- `gte`

to Alya fields like:

- `msgid`
- `errcode`
- `vals`

This keeps validation setup visible and separate from transport logic.

## Logging setup in the WSC example

The WSC example includes minimal LogHarbour logging.

### Startup logging

`cmd/service/main.go` logs:

- database connection initialized
- server starting
- startup failure

### Request logging

Handlers log:

- request receipt
- internal errors

Business-validation failures are returned to clients but not logged as internal errors.

This keeps the example small while showing typed logger injection.

## Setup overview

Both examples follow the same setup pattern.

### Start PostgreSQL

For the REST example:

```bash
cd examples/rest-usersvc-sqlc-example
docker compose up -d
```

For the WSC example:

```bash
cd examples/wsc-usersvc-sqlc-example
docker compose up -d
```

### Run migrations

For the REST example:

```bash
cd examples/rest-usersvc-sqlc-example/pg/migrations
tern migrate
```

For the WSC example:

```bash
cd examples/wsc-usersvc-sqlc-example/internal/db/migrations
tern migrate
```

### Run the service

For the REST example:

```bash
cd examples/rest-usersvc-sqlc-example
go run .
```

For the WSC example:

```bash
cd examples/wsc-usersvc-sqlc-example
go run ./cmd/service
```

## Ports

- REST example: `:8083`
- WSC example: `:8084`

## Routes

Both examples expose the same routes.

Users:

- `POST /users`
- `GET /users`
- `GET /users/{id}`
- `PATCH /users/{id}`
- `DELETE /users/{id}`

Orders:

- `POST /orders`
- `GET /orders`
- `GET /orders/{id}`
- `PATCH /orders/{id}`
- `DELETE /orders/{id}`

## How to choose between them

Use the REST example when:

- you want HTTP-native APIs
- direct JSON bodies are acceptable
- clients should use problem-style error responses

Use the WSC example when:

- you need Alya envelope compatibility
- clients already depend on `status`, `data`, and `messages`
- clients already use Alya `msgid` and `errcode`

## Design summary

### Same internal layers

Both examples keep the same internal architecture.

Reason:
- transport style should not force a different business-logic structure

### Different transport packages

Each example uses a transport package that matches its wire format.

Reason:
- one package should not mix two incompatible response models

### Typed dependencies in both examples

Neither example passes a runtime service object into handlers.

Reason:
- handlers should declare their dependencies explicitly
- constructor injection gives compile-time safety

### Local SQLC assets in both examples

Each example keeps its own database assets inside the example directory.

Reason:
- examples should be runnable on their own
- local docs should match local files
