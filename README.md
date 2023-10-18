# Web Services Framework

## Introduction

Creating a new web service:

- Create a new package under `internal/webservices`
- Create a function `RegisterHandlers` in the package that registers the handlers for the web service
- Handler function
  - Bind request body to struct
  - Validate request body
    - Validate request body using standard validator
    - Add request-specific vals to validation errors
    - Check request specific custom validations and add errors
  - If there are validation errors, add them to response and send it
  - If there are no validation errors, send success response

## Directory Structure

```
/internal (internal packages)
    /webservices (web services handlers)
        /users (user handlers and user-specific utilities)
        /groups
    /wscutils (web services common utilities)
/pkg (public packages)
```





## Working with RDBMS

- We will use sqlc (https://sqlc.dev/). It is a tool that generates type-safe Go code from SQL queries. 
- We use tern for schema migrations (https://github.com/jackc/tern)

- Database schema is present in `internal/pg/migrations/schema.sql`
- Queries are present in `internal/pg/queries.sql`

- To generate code from queries, run `make sqlc-generate`
- To run schema migrations, run `make pg-migrate`

