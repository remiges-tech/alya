package router

import (
	"context"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/logger"
)

const (
	timeout = 60 * time.Second
)

func SetupRouter(useOIDCAuth bool, l logger.Logger, authMiddleware *AuthMiddleware) (*gin.Engine, error) {
	r := gin.Default()

	// Apply timeout middleware globally
	r.Use(TimeoutMiddleware(timeout))

	if useOIDCAuth {
		r.Use(authMiddleware.MiddlewareFunc())
	}

	return r, nil
}

func LoadAuthMiddleware(clientID string, providerURL string, cache TokenCache, l logger.Logger) (*AuthMiddleware, error) {
	// Define a timeout duration
	timeout := 5 * time.Second

	// Create a context with the specified timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		return nil, err
	}

	authMiddleware, err := NewAuthMiddleware(clientID, provider, cache, l)
	if err != nil {
		return nil, err
	}

	return authMiddleware, nil
}
