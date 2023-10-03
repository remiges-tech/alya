package main

import (
	"context"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go-framework/internal/pg"
	"go-framework/internal/pg/sqlc-gen"
	"go-framework/internal/webservices/user"
	voucher "go-framework/internal/webservices/vouchers"
	middleware "go-framework/internal/wscutils"
	"go-framework/logharbour"
	"net/http"
)

var (
	clientID      = "account"
	clientSecret  = "OFyNKbP4g6sYtR4nACtS4V30ILsruzY1"
	keycloakURL   = "https://lemur-7.cloud-iam.com/auth/realms/cool5"
	redisAddr     = "localhost:6379"
	redisPassword = ""
	redisDB       = 0
)

func createEnv() (*sqlc.Queries, *logharbour.LogHarbour, *redis.Client) {
	// Establish Env -- connection connections, logger, etc.
	pgClient := pg.Connect()
	sqlq := sqlc.New(pgClient)
	lh := logharbour.New()

	// Set up Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	return sqlq, lh, rdb
}

func setupMiddleware(rdb *redis.Client) gin.HandlerFunc {
	// Set up Redis cache and Verifier
	redisCache := &middleware.RedisTokenCache{
		Client: rdb,
		Ctx:    context.Background(),
	}

	provider, _ := oidc.NewProvider(context.Background(), keycloakURL)
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})
	authMiddleware := middleware.NewAuthMiddleware(clientID, clientSecret, keycloakURL, verifier, redisCache)

	return authMiddleware.MiddlewareFunc()
}

func main() {
	sqlq, lh, rdb := createEnv()

	// Pass the Env to the handler functions to interact with database
	voucherHandler := voucher.NewHandler(sqlq, lh)
	userHandler := user.NewHandler(sqlq, lh)

	// Setup gin router
	authMiddleware := setupMiddleware(rdb)
	r := gin.Default()
	r.Use(authMiddleware)

	logger := &middleware.CustomLogger{}
	r.Use(middleware.CustomLoggerMiddleware(logger))

	// Register api handlers
	voucherHandler.RegisterVoucherHandlers(r)
	userHandler.RegisterUserHandlers(r)

	// Protected route
	r.GET("/protected", func(c *gin.Context) {
		c.String(http.StatusOK, "This is a protected route")
	})

	r.Run(":8080")
}
