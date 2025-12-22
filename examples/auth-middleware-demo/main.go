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

	// Configure error codes for auth failures
	router.SetDefaultMsgID(9999)
	router.SetDefaultErrCode("AUTH_ERROR")
	router.RegisterAuthMsgID(router.TokenMissing, 1001)
	router.RegisterAuthErrCode(router.TokenMissing, "AUTH_TOKEN_MISSING")
	router.RegisterAuthMsgID(router.TokenVerificationFailed, 1002)
	router.RegisterAuthErrCode(router.TokenVerificationFailed, "AUTH_TOKEN_INVALID")
	router.RegisterAuthMsgID(router.TokenCacheFailed, 1003)
	router.RegisterAuthErrCode(router.TokenCacheFailed, "AUTH_CACHE_ERROR")

	// Setup dependencies
	l := logger.NewLogger(os.Stdout)
	oidcURL := getEnv("OIDC_PROVIDER_URL", "http://localhost:8080/realms/demo")
	clientID := getEnv("CLIENT_ID", "auth-demo-client")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")

	provider, err := oidc.NewProvider(context.Background(), oidcURL)
	if err != nil {
		log.Fatalf("Failed to create OIDC provider: %v", err)
	}

	cache := router.NewRedisTokenCache(redisAddr, "", 0, 0)

	authMW, err := router.NewAuthMiddlewareWithConfig(router.AuthMiddlewareConfig{
		ClientID:  clientID,
		Provider:  router.WrapOIDCProvider(provider),
		Cache:     cache,
		Logger:    l,
		IssuerURL: oidcURL,
	})
	if err != nil {
		log.Fatalf("Failed to create auth middleware: %v", err)
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	api.Use(authMW.MiddlewareFunc())
	{
		api.GET("/user", func(c *gin.Context) {
			userID, _ := c.Get("user_id")
			email, _ := c.Get("email")
			claims, _ := c.Get("jwt_claims")
			c.JSON(200, gin.H{
				"user_id": userID,
				"email":   email,
				"claims":  claims,
			})
		})

		api.GET("/data", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "This is protected data",
				"data":    []string{"item1", "item2", "item3"},
			})
		})
	}

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
