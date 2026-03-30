# Directory structure for client tech leaders

Theme: start with an open structure, then split by business domain when the service grows.

This note explains the directory structure we are using now, and how we expect it to evolve later.

By business domain, we mean an area such as `users`, `orders`, or `payments`.

## Current approach

We are starting with an open structure.

```text
project/
  cmd/
    service/
      main.go

  internal/
    api/
    http/
    service/
    repository/
    db/
    validation/
    shared/

  pkg/        optional

  go.mod
  go.sum
  README.md
```

This means the code is grouped first by concern:

- `api/` for request and response types
- `http/` for handlers and routes
- `service/` for business logic
- `repository/` for persistence contracts and implementations
- `db/` for database setup, migrations, SQL files, SQLC config, and generated code
- `validation/` for common validation setup
- `shared/` for reusable helper code inside this service

This is a good fit for the current phase.

The service is still small enough that the team can work across the whole codebase without much friction.

It also keeps the structure easy to read while the business flows are still settling.

## What the top-level directories mean

### `cmd/`

`cmd/` holds application entrypoints.

For most services, one entrypoint is enough:

```text
cmd/service/main.go
```

`main.go` should stay small.

It should do startup wiring only:

- load config
- initialize dependencies
- build services and handlers
- register routes
- start the server

It should not hold business logic.

### `internal/`

`internal/` holds code that belongs only to this service.

This is where the application code lives.

Today that code is grouped by concern. Later, if the business domains become clear, parts of `internal/` will be reorganized by domain.

### `pkg/`

`pkg/` is optional.

Use it only for packages that are meant to be imported by other modules or services.

If code is shared only inside this service, keep it under `internal/`, not `pkg/`.

## Why we are starting this way

At the start of a service, business boundaries are often not fully stable.

If we create separate domain directories too early, we may guess the wrong boundaries and reorganize again soon after.

The open structure avoids that early overhead.

It lets us keep the codebase simple while the service shape is still emerging.

## What stays stable from the start

Some parts of the structure should stay stable even if we reorganize later.

### `internal/db/`

Database assets stay together under `internal/db/`:

- connection setup
- migrations
- SQL files
- SQLC config
- generated SQLC code

This is shared infrastructure.

### `internal/validation/`

Common validation setup stays here while it is shared across the service.

### `internal/shared/`

`shared/` is similar to a local `utils` directory.

Use it for helper code reused inside this service.

Examples:

- common HTTP helpers
- shared response mapping helpers
- common logging helpers

It should not become a dump for unrelated helpers.

If code is used only by one business domain, keep it close to that domain.

## How we expect this to evolve

As the service grows, we expect clearer business domains to emerge.

When that happens, we will split the code into domain directories.

A later structure may look like this:

```text
project/
  cmd/
    service/
      main.go

  internal/
    users/
      api/
      http/
      service/
      repository/

    orders/
      api/
      http/
      service/
      repository/

    db/
      pg.go
      migrations/
      queries/
      sqlc/
      sqlc.yaml

    validation/
    shared/

  pkg/        optional

  go.mod
  go.sum
  README.md
```

At that stage, code for one business domain stays close together.

A change to `users` usually stays under `internal/users/`. A change to `orders` usually stays under `internal/orders/`.

## When we should make that change

We should split by business domain when the boundaries become real in day-to-day development.

Typical signs are:

- business areas start changing independently
- one feature change touches many top-level directories
- different teams start owning different areas
- reviews become harder because unrelated code is mixed together

## Summary

The current structure is intentionally open.

It keeps the service simple while the design is still settling.

Later, if the business domains grow and stabilize, we will reorganize around those domains.

This gives us a straightforward starting point now and a clear path for scaling later.
