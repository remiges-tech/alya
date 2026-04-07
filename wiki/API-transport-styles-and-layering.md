# API transport styles and layering

This note explains the transport changes added to Alya and the design decisions behind them.

Theme: keep the internal service structure the same, and let only the HTTP transport contract change.

## What changed

Alya now has two transport styles:

- `restutils` for modern REST-style web services
- `wscutils` for Alya envelope style web services

Both styles use the same internal layering:

```text
transport -> app -> repository -> sqlc -> database
```

Both styles also use the same dependency model:

- typed constructor injection
- no runtime dependency bag in handlers
- no `any` type assertions in handlers

## Why this split exists

Alya already had one established wire format:

- request body wrapped in `data`
- response body wrapped in `status`, `data`, and `messages`

That format is in use by existing services. It cannot be changed lightly.

At the same time, some new services need a more HTTP-native style:

- direct JSON request bodies
- plain JSON resource responses
- problem-details style error responses
- stronger use of HTTP status codes

Instead of forcing one transport style on all services, Alya now supports both.

## What stays the same in both styles

The transport style changes only the HTTP contract. It does not change the internal application structure.

The same rules apply in both examples:

- handlers call app services
- app services call repositories
- repositories use SQLC
- dependencies are passed through constructors
- handlers do not pull dependencies out of a runtime object

This keeps transport decisions separate from business logic and database access.

## `restutils`

Use `restutils` for modern REST-style services.

### Request style

Request bodies are sent directly.

Example:

```json
{
  "name": "Sachin",
  "email": "sachin@example.com",
  "username": "sachin"
}
```

### Response style

Success responses return resources directly.

Example:

```json
{
  "id": 1,
  "name": "Sachin",
  "email": "sachin@example.com",
  "username": "sachin"
}
```

Error responses use a problem-details style shape.

### Helpers

`restutils` provides helpers for:

- direct JSON body binding
- JSON field validation errors
- plain JSON success responses
- problem-style error responses

## `wscutils`

Use `wscutils` for Alya envelope style services.

### Request style

Request bodies keep the existing Alya envelope.

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

### Response style

Responses keep the existing Alya envelope.

Success:

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

Error:

```json
{
  "status": "error",
  "data": {},
  "messages": [
    {
      "msgid": 45,
      "errcode": "missing",
      "field": "email"
    }
  ]
}
```

### Existing and additive API

`wscutils` keeps its existing API for compatibility.

New additive APIs were added for new envelope-style services:

- `BindData`
- `ParseInt64PathParam`
- `Validator`
- `ValidationRule`

These are additive. Existing `wscutils` users do not need to change.

Response construction stays on the existing `wscutils` response API.

## Why `wscutils.Validator` was added

The existing `WscValidate` API uses package-level validation maps. That works, but it creates global state.

The new `wscutils.Validator` is instance-based.

It solves three problems:

1. validation setup is local to one service or module
2. field names come from JSON tags, not Go struct field names
3. it directly returns Alya `ErrorMessage` values with `msgid`, `errcode`, `field`, and `vals`

This keeps the Alya error model while removing global validation state for new code.

## Why we do not use `service.Service` in handlers for new examples

Alya has an existing `service.Service` type that wraps a Gin router and stores dependencies. That design is still available, and existing code can keep using it.

The new examples do not pass `service.Service` into handlers.

Reason:

- handlers should not receive one runtime object and pull typed dependencies out of it
- that pattern leads to hidden dependencies and runtime type assertions
- typed constructor injection gives compile-time safety

In the new examples, handlers receive only the dependencies they actually use.

Example:

```go
type UserHandler struct {
    app       *app.UserService
    validator *wscutils.Validator
    logger    *logharbour.Logger
}
```

## Why we do not add a framework-wide container package

A small local wiring container can help in `main.go` if startup gets large.

A framework-wide runtime container is not part of this design.

Reason:

- it would become another dependency bag
- it would push Alya back toward service-locator style code
- it would weaken the explicit dependency model

The preferred pattern is:

- build dependencies in `main.go`
- inject them through constructors
- store typed fields on handlers and app services

## Logging

The new WSC example includes minimal LogHarbour logging.

The logging rules are basic:

- log server startup
- log request receipt
- log internal errors
- do not log normal business validation failures as internal errors

This keeps the example small while showing typed logger injection.

## Resource-oriented routes in both styles

Both transport styles use resource-oriented routes.

Example:

- `POST /users`
- `GET /users`
- `GET /users/{id}`
- `PATCH /users/{id}`
- `DELETE /users/{id}`

The difference is the wire format, not the route structure.

## Design decisions summary

### Decision: support both transport styles

Reason:
- existing Alya envelope clients must keep working
- new services may want HTTP-native REST style

### Decision: keep the same app/repository layering in both

Reason:
- business logic should not depend on transport format
- repositories should not depend on response envelopes

### Decision: use typed constructor injection

Reason:
- compile-time safety
- clear dependencies
- easier tests

### Decision: keep `wscutils` additive, not breaking

Reason:
- Alya is already used in production
- old code should not be forced onto a new API

### Decision: keep `restutils` and `wscutils` separate

Reason:
- they represent different HTTP contracts
- one package should not mix two response models

## Choosing between `restutils` and `wscutils`

Use `restutils` when:

- you want direct JSON bodies
- you want plain JSON resource responses
- you want problem-style error responses
- you want a more HTTP-native API

Use `wscutils` when:

- you need Alya's existing request and response envelope
- clients already depend on `msgid`, `errcode`, `field`, and `vals`
- you want to keep wire compatibility with older Alya services

## What these changes do not change

These changes do not redefine Alya as a new web framework.

Gin remains the HTTP framework underneath.

Alya remains:

- a service-development layer on top of Gin
- a set of conventions and utility packages
- an integration point for validation, logging, config, SQLC, and jobs
