# Auth Middleware Demo

JWT authentication with Keycloak using Alya's auth middleware.

## Quick Start

```bash
docker-compose up -d  # Wait 60 seconds for Keycloak to initialize
go run main.go        # Server runs on http://localhost:8083
```

## Testing

Import `postman-collection.json` into Postman and run the requests in order:

1. **Get Token** - Authenticates and saves token
2. **Health Check** - Public endpoint
3. **Missing Token Error** - Returns AUTH_TOKEN_MISSING (msgid: 1001)
4. **Invalid Token Error** - Returns AUTH_TOKEN_INVALID (msgid: 1002)
5. **Get User Info** - Protected, returns JWT claims
6. **Get Protected Data** - Protected endpoint

## Credentials

- **Keycloak Admin**: http://localhost:8080 (admin/admin)
- **Test User**: testuser / testpass123

## Routes

| Route | Auth | Description |
|-------|------|-------------|
| GET /health | No | Health check |
| GET /api/user | Yes | Returns JWT claims |
| GET /api/data | Yes | Returns sample data |

## Claims in Context

After authentication, handlers can access:

```go
userID, _ := c.Get("user_id")     // Subject claim
email, _ := c.Get("email")        // Email claim
claims, _ := c.Get("jwt_claims")  // All claims
```

## Troubleshooting

**"expected audience X got []"**: Add audience mapper in Keycloak client settings.

**"token missing required claim: sub"**: Add subject mapper in OIDC provider.

**"Failed to create OIDC provider"**: Check Keycloak is running (`docker-compose ps`).

**"AUTH_CACHE_ERROR"**: Check Redis is running (`docker-compose ps redis`).

## Stop

```bash
docker-compose down
```
