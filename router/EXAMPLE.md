# Secure Auth Middleware - Usage Examples

This file provides practical examples for using the VAPT-compliant authentication middleware.

## Table of Contents

1. [Basic Setup](#basic-setup)
2. [Strict Mode (Recommended)](#strict-mode-recommended)
3. [Custom Claims Validation](#custom-claims-validation)
4. [Migration from Legacy Code](#migration-from-legacy-code)
5. [Full Production Example](#full-production-example)

---

## Basic Setup

### Step 1: Initialize OIDC Provider

```go
package main

import (
    "context"
    "log"

    "github.com/coreos/go-oidc/v3/oidc"
    "github.com/remiges-tech/alya/router"
)

func main() {
    ctx := context.Background()

    // Initialize OIDC provider
    provider, err := oidc.NewProvider(ctx, "https://keycloak.example.com/realms/myrealm")
    if err != nil {
        log.Fatal(err)
    }

    // ... continue below
}
```

### Step 2: Create Token Cache

```go
// Using Redis token cache
tokenCache := router.NewRedisTokenCache(
    "localhost:6379", // Redis address
    "",               // Password (if any)
    0,                // DB number
    30 * time.Second, // Expiration
)
```

### Step 3: Create Secure Auth Middleware

```go
// Create VAPT-compliant middleware
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    ClientID:  "my-app-client",
    Provider:  provider,
    Cache:     tokenCache,
    Logger:    logger,
    IssuerURL: "https://keycloak.example.com/realms/myrealm",
})
if err != nil {
    log.Fatal(err)
}
```

### Step 4: Apply to Gin Router

```go
r := gin.Default()

// Apply middleware globally
r.Use(authMW.MiddlewareFunc())

// All routes below require valid JWT
r.POST("/api/data", handleData)
r.GET("/api/users", handleUsers)
```

---

## Strict Mode (Recommended)

### Maximum Security Configuration

```go
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    ClientID:  "my-app",
    Provider:  provider,
    Cache:     tokenCache,
    Logger:    logger,
    IssuerURL: "https://keycloak.example.com/realms/myrealm",

    // Explicit strict mode (this is already the default)
    SecurityMode: router.StrictMode,

    // Only allow RS256 (most restrictive)
    AllowedAlgorithms: []string{"RS256"},

    // Require additional claims
    RequiredClaims: []string{
        "exp",   // Expiration
        "iss",   // Issuer
        "sub",   // Subject
        "email", // Email (custom requirement)
        "roles", // Roles (custom requirement)
    },

    // Store claims in Gin context
    StoreClaimsInContext: true,
})
```

---

## Custom Claims Validation

### Example 1: Validate User Roles

```go
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    ClientID:  "my-app",
    Provider:  provider,
    Cache:     tokenCache,
    Logger:    logger,
    IssuerURL: "https://keycloak.example.com/realms/myrealm",

    ValidateClaimsFunc: func(claims jwt.MapClaims) error {
        // Ensure user has at least one role
        roles, ok := claims["roles"].([]interface{})
        if !ok || len(roles) == 0 {
            return fmt.Errorf("user must have at least one role")
        }

        // Ensure user is not disabled
        if disabled, ok := claims["disabled"].(bool); ok && disabled {
            return fmt.Errorf("user account is disabled")
        }

        return nil
    },
})
```

### Example 2: Validate Email Domain

```go
ValidateClaimsFunc: func(claims jwt.MapClaims) error {
    email, ok := claims["email"].(string)
    if !ok {
        return fmt.Errorf("email claim required")
    }

    // Only allow company email addresses
    if !strings.HasSuffix(email, "@company.com") {
        return fmt.Errorf("only company email addresses allowed")
    }

    return nil
},
```

### Example 3: Validate Organization

```go
ValidateClaimsFunc: func(claims jwt.MapClaims) error {
    orgID, ok := claims["org_id"].(string)
    if !ok || orgID == "" {
        return fmt.Errorf("org_id claim required")
    }

    orgType, ok := claims["org_type"].(string)
    if !ok || orgType == "" {
        return fmt.Errorf("org_type claim required")
    }

    // Validate against allowed organizations
    allowedOrgs := map[string]bool{
        "org-123": true,
        "org-456": true,
    }

    if !allowedOrgs[orgID] {
        return fmt.Errorf("organization %s not authorized", orgID)
    }

    return nil
},
```

---

## Migration from Legacy Code

### Before (Insecure)

```go
// Old way - NOT SECURE
authMW, err := router.NewAuthMiddleware(
    "my-client",
    provider,
    redisCache,
    logger,
)
r.Use(authMW.MiddlewareFunc())
```

### After (Secure)

```go
// New way - SECURE
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    ClientID:  "my-client",
    Provider:  provider,
    Cache:     redisCache,
    Logger:    logger,
    IssuerURL: "https://keycloak.example.com/realms/myrealm", // Required!
})
r.Use(authMW.MiddlewareFunc())
```

### Gradual Migration (Temporary)

If you need time to migrate, you can temporarily use compatibility mode:

```go
// Temporary compatibility mode during migration
authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
    ClientID:     "my-client",
    Provider:     provider,
    Cache:        redisCache,
    Logger:       logger,
    IssuerURL:    "https://keycloak.example.com/realms/myrealm",
    SecurityMode: router.CompatibilityMode, // ⚠️ Temporary only!
})

// TODO: Switch to StrictMode after testing
```

---

## Full Production Example

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/coreos/go-oidc/v3/oidc"
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
    "github.com/remiges-tech/alya/logger"
    "github.com/remiges-tech/alya/router"
)

func main() {
    ctx := context.Background()

    // 1. Initialize logger
    appLogger := logger.NewLogger(nil)

    // 2. Initialize OIDC provider
    keycloakURL := "https://keycloak.example.com/realms/production"
    provider, err := oidc.NewProvider(ctx, keycloakURL)
    if err != nil {
        log.Fatalf("Failed to initialize OIDC provider: %v", err)
    }

    // 3. Create Redis token cache
    tokenCache := router.NewRedisTokenCache(
        "redis:6379",     // Production Redis
        "your-password",  // Redis password
        0,                // DB 0
        30*time.Second,   // 30 second cache
    )

    // 4. Create secure auth middleware with custom validation
    authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
        ClientID:  "production-app",
        Provider:  provider,
        Cache:     tokenCache,
        Logger:    appLogger,
        IssuerURL: keycloakURL,

        // Security settings
        SecurityMode:      router.StrictMode,
        AllowedAlgorithms: []string{"RS256"},

        // Required claims
        RequiredClaims: []string{
            "exp",
            "iss",
            "sub",
            "email",
            "org_id",
            "roles",
        },

        // Custom validation logic
        ValidateClaimsFunc: func(claims jwt.MapClaims) error {
            // Validate email domain
            email, _ := claims["email"].(string)
            if !strings.HasSuffix(email, "@company.com") {
                return fmt.Errorf("invalid email domain")
            }

            // Validate roles
            roles, ok := claims["roles"].([]interface{})
            if !ok || len(roles) == 0 {
                return fmt.Errorf("user must have roles")
            }

            // Validate organization
            orgID, _ := claims["org_id"].(string)
            if orgID == "" {
                return fmt.Errorf("org_id required")
            }

            return nil
        },

        // Store claims for handlers to use
        StoreClaimsInContext: true,
    })
    if err != nil {
        log.Fatalf("Failed to create auth middleware: %v", err)
    }

    // 5. Setup Gin router
    r := gin.Default()

    // 6. Public routes (no authentication)
    r.GET("/health", healthHandler)
    r.GET("/version", versionHandler)

    // 7. Protected routes (authentication required)
    r.Use(authMW.MiddlewareFunc())
    {
        r.POST("/api/users", createUserHandler)
        r.GET("/api/users/:id", getUserHandler)
        r.PUT("/api/users/:id", updateUserHandler)
        r.DELETE("/api/users/:id", deleteUserHandler)

        r.POST("/api/data", createDataHandler)
        r.GET("/api/data", getDataHandler)
    }

    // 8. Start server
    log.Println("Server starting on :8080")
    if err := r.Run(":8080"); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}

// Handler examples

func healthHandler(c *gin.Context) {
    c.JSON(200, gin.H{"status": "healthy"})
}

func versionHandler(c *gin.Context) {
    c.JSON(200, gin.H{"version": "1.0.0"})
}

func createUserHandler(c *gin.Context) {
    // Get verified claims from context
    claims, _ := c.Get("jwt_claims")
    jwtClaims := claims.(jwt.MapClaims)

    // Get user ID from claims
    userID, _ := c.Get("user_id")
    email, _ := c.Get("email")

    log.Printf("User %s (%s) creating new user", userID, email)

    // Check roles from claims
    roles := jwtClaims["roles"].([]interface{})
    hasAdminRole := false
    for _, role := range roles {
        if role.(string) == "admin" {
            hasAdminRole = true
            break
        }
    }

    if !hasAdminRole {
        c.JSON(403, gin.H{"error": "admin role required"})
        return
    }

    // Process request...
    c.JSON(200, gin.H{"message": "user created"})
}

func getUserHandler(c *gin.Context) {
    // All authenticated requests have access to verified claims
    userID, _ := c.Get("user_id")
    claims, _ := c.Get("jwt_claims")

    c.JSON(200, gin.H{
        "user_id": userID,
        "claims":  claims,
    })
}

func updateUserHandler(c *gin.Context) {
    // Handler implementation
    c.JSON(200, gin.H{"message": "user updated"})
}

func deleteUserHandler(c *gin.Context) {
    // Handler implementation
    c.JSON(200, gin.H{"message": "user deleted"})
}

func createDataHandler(c *gin.Context) {
    // Handler implementation
    c.JSON(200, gin.H{"message": "data created"})
}

func getDataHandler(c *gin.Context) {
    // Handler implementation
    c.JSON(200, gin.H{"message": "data retrieved"})
}
```

---

## Testing Your Setup

### Test 1: Valid Token

```bash
# Get a valid token from Keycloak
TOKEN="your-jwt-token-here"

# Test authenticated endpoint
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/users
```

### Test 2: Invalid Token (Should Fail)

```bash
# Test with invalid token
curl -H "Authorization: Bearer invalid.token.here" \
     http://localhost:8080/api/users

# Expected: 401 Unauthorized
```

### Test 3: No Token (Should Fail)

```bash
# Test without token
curl http://localhost:8080/api/users

# Expected: 401 Unauthorized - Missing token
```

### Test 4: Public Endpoint (Should Work)

```bash
# Test public endpoint
curl http://localhost:8080/health

# Expected: 200 OK
```

---

## Troubleshooting

### Problem: "JWT verifier not initialized"

**Solution:** Ensure `IssuerURL` is provided in strict mode:
```go
IssuerURL: "https://keycloak.example.com/realms/myrealm",
```

### Problem: "Algorithm not allowed"

**Solution:** Check your Keycloak configuration matches allowed algorithms:
```go
AllowedAlgorithms: []string{"RS256", "RS384", "RS512"},
```

### Problem: "Token missing required claim"

**Solution:** Verify your Keycloak client includes all required claims:
```go
RequiredClaims: []string{"exp", "iss", "sub", "email"},
```

### Problem: "Custom claims validation failed"

**Solution:** Debug your custom validator:
```go
ValidateClaimsFunc: func(claims jwt.MapClaims) error {
    // Add debug logging
    fmt.Printf("Claims: %+v\n", claims)

    // Your validation logic...
    return nil
},
```

---

## Best Practices Summary

✅ **DO:**
- Use `router.StrictMode`
- Specify `IssuerURL`
- Use RSA algorithms only (`RS256`, `RS384`, `RS512`)
- Validate required claims
- Implement custom validators for business logic
- Store claims in context
- Test with both valid and invalid tokens

❌ **DON'T:**
- Use `router.CompatibilityMode` in production
- Allow weak algorithms (`HS256`, `none`)
- Skip security checks
- Trust client-side token modifications
- Ignore token validation errors

---

**Need Help?** Check the [SECURITY.md](./SECURITY.md) documentation for more details.
