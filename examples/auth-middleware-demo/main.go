package main

import (
	"context"
	"log"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/remiges-tech/alya/logger"
	"github.com/remiges-tech/alya/router"
)

func main() {
	_ = godotenv.Load()

	// ============================================================================
	// Step 1: Configure Error Codes (REQUIRED)
	// ============================================================================
	// SECURITY: Standardized error codes prevent information leakage and enable
	// consistent error handling. Each auth failure scenario gets a specific
	// msgid and errcode for monitoring and client handling.

	router.SetDefaultMsgID(9999)
	router.SetDefaultErrCode("AUTH_ERROR")

	// TokenMissing: No Authorization header present
	router.RegisterAuthMsgID(router.TokenMissing, 1001)
	router.RegisterAuthErrCode(router.TokenMissing, "AUTH_TOKEN_MISSING")

	// TokenVerificationFailed: Invalid signature, expired, wrong audience, etc.
	router.RegisterAuthMsgID(router.TokenVerificationFailed, 1002)
	router.RegisterAuthErrCode(router.TokenVerificationFailed, "AUTH_TOKEN_INVALID")

	// TokenCacheFailed: Redis connection or caching errors
	router.RegisterAuthMsgID(router.TokenCacheFailed, 1003)
	router.RegisterAuthErrCode(router.TokenCacheFailed, "AUTH_CACHE_ERROR")

	// ============================================================================
	// Step 2: Setup Dependencies
	// ============================================================================

	l := logger.NewLogger(os.Stdout)
	oidcURL := getEnv("OIDC_PROVIDER_URL", "http://localhost:8080/realms/demo")
	clientID := getEnv("CLIENT_ID", "auth-demo-client")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")

	// OIDC Provider: Fetches public keys and validates tokens
	// Works with Keycloak, Auth0, Okta, or any OIDC-compliant provider
	provider, err := oidc.NewProvider(context.Background(), oidcURL)
	if err != nil {
		log.Fatalf("Failed to create OIDC provider: %v", err)
	}

	// PERFORMANCE: Redis caching reduces OIDC provider calls by caching validated tokens
	// Format: NewRedisTokenCache(addr, password, db, maxRetries)
	cache := router.NewRedisTokenCache(redisAddr, "", 0, 0)

	// ============================================================================
	// Step 3: Create VAPT-Compliant Auth Middleware
	// ============================================================================
	// SECURITY: StrictMode enables VAPT compliance with:
	// - Algorithm whitelisting (RS256, RS384, RS512 only)
	// - Comprehensive claims validation (exp, iss, nbf, iat, sub, aud)
	// - Clock skew protection (5 second grace period)
	// - Issuer and audience verification
	//
	// Use CompatibilityMode only for legacy systems that can't provide all claims.

	authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
		ClientID:     clientID,                          // Expected audience in JWT
		Provider:     router.WrapOIDCProvider(provider), // OIDC provider for key verification
		Cache:        cache,                             // Redis cache (optional, can be nil)
		Logger:       l,                                 // Logger instance
		IssuerURL:    oidcURL,                           // Expected issuer in JWT
		SecurityMode: router.StrictMode,                 // VAPT-compliant validation (recommended)
	})
	if err != nil {
		log.Fatalf("Failed to create auth middleware: %v", err)
	}

	// ============================================================================
	// Step 4: Setup Routes
	// ============================================================================

	r := gin.Default()

	// Public route - no authentication required
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Protected routes - authentication required
	// All routes under /api will validate JWT tokens
	api := r.Group("/api")
	api.Use(authMW.MiddlewareFunc()) // Apply auth middleware to all routes in this group
	{
		// Example: Access JWT claims extracted by middleware
		// Available claims: user_id (sub), email, jwt_claims (all claims)
		api.GET("/user", func(c *gin.Context) {
			userID, _ := c.Get("user_id")    // Subject claim (sub)
			email, _ := c.Get("email")       // Email claim
			claims, _ := c.Get("jwt_claims") // All JWT claims as map[string]any

			c.JSON(200, gin.H{
				"user_id": userID,
				"email":   email,
				"claims":  claims,
			})
		})

		// Example: Simple protected endpoint
		api.GET("/data", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "This is protected data",
				"data":    []string{"item1", "item2", "item3"},
			})
		})
	}

	// Start server
	port := getEnv("PORT", "8083")
	log.Printf("Server starting on :%s", port)
	log.Printf("OIDC Provider: %s", oidcURL)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
