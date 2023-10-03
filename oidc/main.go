package main

import (
	"context"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	middleware "go-framework/internal/wscutils"
)

var (
	clientID      = "account"
	clientSecret  = "OFyNKbP4g6sYtR4nACtS4V30ILsruzY1"
	keycloakURL   = "https://lemur-7.cloud-iam.com/auth/realms/cool5"
	redisAddr     = "localhost:6379"
	redisPassword = ""
	redisDB       = 0
)

func main() {
	r := gin.Default()

	// Setup redis client and verifier
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	redisCache := &middleware.RedisTokenCache{
		Client: rdb,
		Ctx:    context.Background(),
	}
	provider, err := oidc.NewProvider(context.Background(), keycloakURL)
	if err != nil {
		// need to handle err
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	authMiddleware := middleware.NewAuthMiddleware(clientID, clientSecret, keycloakURL, verifier, redisCache)

	r.Use(authMiddleware.MiddlewareFunc())

	r.GET("/protected", func(c *gin.Context) {
		c.String(http.StatusOK, "This is a protected route")
	})

	r.Run(":8080")
}
