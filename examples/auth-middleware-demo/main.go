package main

import (
	"context"
	"log"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/remiges-tech/alya/logger"
	"github.com/remiges-tech/alya/router"
)

func main() {
	// ========================================
	// STEP 1: Configure Error Codes
	// ========================================
	// This MUST be done BEFORE creating the middleware
	setupAuthErrorCodes()

	// ========================================
	// STEP 2: Initialize Dependencies
	// ========================================
	l := logger.New(logger.Config{})

	// Get configuration from environment
	oidcProviderURL := getEnv("OIDC_PROVIDER_URL", "https://keycloak.example.com/realms/myrealm")
	clientID := getEnv("CLIENT_ID", "your-client-id")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")

	// Create OIDC provider
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, oidcProviderURL)
	if err != nil {
		log.Fatalf("Failed to create OIDC provider: %v", err)
	}

	// Create token cache
	cache := router.NewRedisTokenCache(redisAddr, "", 0, 0)

	// ========================================
	// STEP 3: Create Auth Middleware
	// ========================================
	authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
		ClientID:     clientID,
		Provider:     router.WrapOIDCProvider(provider),
		Cache:        cache,
		Logger:       l,
		IssuerURL:    oidcProviderURL,
		SecurityMode: router.StrictMode, // VAPT-compliant security

		// Optional: Custom validation for business logic
		ValidateClaimsFunc: validateBusinessRules,
	})
	if err != nil {
		log.Fatalf("Failed to create auth middleware: %v", err)
	}

	// ========================================
	// STEP 4: Setup Router
	// ========================================
	r := setupRouter(authMW)

	// ========================================
	// STEP 5: Start Server
	// ========================================
	port := getEnv("PORT", "8080")
	log.Printf("Server starting on :%s", port)
	log.Printf("OIDC Provider: %s", oidcProviderURL)
	log.Printf("Redis Cache: %s", redisAddr)
	log.Printf("Security Mode: StrictMode (VAPT-compliant)")

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// setupAuthErrorCodes configures standardized error codes for auth failures
func setupAuthErrorCodes() {
	// Set default fallback values
	router.SetDefaultMsgID(9999)
	router.SetDefaultErrCode("AUTH_ERROR")

	// Register specific error scenarios with meaningful codes
	router.RegisterAuthMsgID(router.TokenMissing, 1001)
	router.RegisterAuthErrCode(router.TokenMissing, "AUTH_TOKEN_MISSING")

	router.RegisterAuthMsgID(router.TokenVerificationFailed, 1002)
	router.RegisterAuthErrCode(router.TokenVerificationFailed, "AUTH_TOKEN_INVALID")

	router.RegisterAuthMsgID(router.TokenCacheFailed, 1003)
	router.RegisterAuthErrCode(router.TokenCacheFailed, "AUTH_CACHE_ERROR")

	log.Println("✓ Auth error codes configured")
}

// validateBusinessRules is a custom claims validator for business-specific logic
func validateBusinessRules(claims jwt.MapClaims) error {
	// Example: You can add custom validation here
	// Uncomment to require specific role:
	/*
		role, ok := claims["role"].(string)
		if !ok || role != "admin" {
			return fmt.Errorf("admin role required")
		}
	*/

	return nil // Allow all valid tokens
}

// setupRouter configures Gin routes with auth middleware
func setupRouter(authMW *router.AuthMiddleware) *gin.Engine {
	r := gin.Default()

	// ========================================
	// Public Routes (No Authentication)
	// ========================================
	r.GET("/health", healthCheck)
	r.GET("/", homepage)

	// ========================================
	// Protected Routes (Authentication Required)
	// ========================================
	api := r.Group("/api")
	api.Use(authMW.MiddlewareFunc()) // Apply auth middleware
	{
		api.GET("/user", getUserInfo)
		api.GET("/users", listUsers)
		api.POST("/users", createUser)
	}

	return r
}

// ========================================
// Route Handlers
// ========================================

func healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "ok",
		"service": "auth-middleware-demo",
	})
}

func homepage(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "Auth Middleware Demo API",
		"endpoints": gin.H{
			"health":  "/health",
			"api":     "/api/*",
		},
		"authentication": "Bearer token required for /api/* routes",
	})
}

func getUserInfo(c *gin.Context) {
	// Extract validated claims from context
	claims, exists := c.Get("jwt_claims")
	if !exists {
		c.JSON(500, gin.H{"error": "Claims not found in context"})
		return
	}

	// Extract convenient user info
	userID, _ := c.Get("user_id")
	email, _ := c.Get("email")
	username, _ := c.Get("username")

	c.JSON(200, gin.H{
		"user_id":  userID,
		"email":    email,
		"username": username,
		"claims":   claims,
	})
}

func listUsers(c *gin.Context) {
	// This is a protected endpoint
	c.JSON(200, gin.H{
		"users": []gin.H{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		},
	})
}

func createUser(c *gin.Context) {
	// This is a protected endpoint
	var req struct {
		Name  string `json:"name" binding:"required"`
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, gin.H{
		"message": "User created",
		"user": gin.H{
			"name":  req.Name,
			"email": req.Email,
		},
	})
}

// ========================================
// Helpers
// ========================================

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
