package router

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/logger"
)

func SetupRouter(useOIDCAuth bool, l logger.Logger, authMiddleware *AuthMiddleware) (*gin.Engine, error) {
	r := gin.Default()

	if useOIDCAuth {
		r.Use(authMiddleware.MiddlewareFunc())
	}

	return r, nil
}

func LoadAuthMiddleware(clientID string, providerURL string, cache TokenCache, l logger.Logger) (*AuthMiddleware, error) {
	ctx := context.Background()
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
