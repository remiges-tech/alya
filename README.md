# Alya
# Alya Framework

## Introduction

Alya is an open-source web services development framework for Go programmers. It is built on top of [Gin framework](https://gin-gonic.com/). It provides a set of tools, libraries, and conventions to help developers rapidly build secure, scalable, and maintainable web services. Alya aims to streamline common tasks and provide a standardized approach to building web services.

Brief introduction of packages in Alya:

- `wscutils`: It contains utility functions for web service development, such as request validation, response building, and error handling.
- `service`: It provides a service abstraction to create modular services with dependencies required for the service.
- `jobs`: It provides a framework for batch processing and slow queries (long-running tasks).

## Core Concepts

### Service

In Alya, a `service` represents a modular component of the application, abstracting related functionality. It provides a way to organize and structure the codebase.

A router is responsible for handling HTTP requests and routing them to the appropriate handlers. 

Alya `service` uses `router` to register its routes and handlers. This allows service to define its own routes and handlers independently. With service abstraction we can have different route related logic for each service like different middleware for each service (to be implemented).

Creating a service: 

```
r := gin.Default() // creates a router
userService := service.NewService(r) // creates a service
userService.RegisterRoute(http.MethodPost, "/users", usersvc.HandleCreateUserRequest) // registers a route for the service
```

### Request handling

There are following steps in request handling

1. bind request body to struct

    Use `wscutils.BindJSON` to bind request body to struct.

2. validate request body	

    2.1. validate request body using standard validator

    Use `wscutils.WscValidate` to validate request body using standard validators -- it uses `go-playground/validator` to validate the struct. 

    2.2. add request-specific vals to validation errors

    Read about [Alya web service response format](https://github.com/remiges-tech/alya/wiki/Web-services-design-standards#web-service-response-format). We have field and vals in the response. `WscValidate` being a generic function is not aware of vals to be sent for a specific field. So, it expects a function `getVals func(err validator.FieldError) []string` to be passed. This function should return a list of vals to be sent for a specific field.

    2.3. check request specific custom validations and add errors

    For request specific custom validations where some custom business logic is required, then write those functions and ensure that validation errors are appended to `[]wscutils.ErrorMessage`

3. if there are validation errors, add them to response and send it

    Return the response using `wscutils.SendErrorResponse`

4. process the request

    If there are  errors, send error response, else send success response using `wscutils.SendSuccessResponse`

### Response formatting 

### Error handling

### Batch processing and slow queries

## Authentication and Authorization

### OAuth2 and token-based authentication

### Integration with IDShield or Keycloak


## Batch Processing
- Registering initializers and processors
- Submitting and tracking batch jobs and slow queries
- Handling output files

## Examples
- Basic API implementation
- Batch processing example
- Authentication and authorization

## License
- Apache License 2.0
