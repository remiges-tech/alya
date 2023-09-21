package main

import (
	"github.com/gin-gonic/gin"
	"go-framework/internal/pg"
	"go-framework/internal/pg/sqlc-gen"
	"go-framework/internal/webservices/user"
	voucher "go-framework/internal/webservices/vouchers"
	"go-framework/internal/wscutils"
	"go-framework/logharbour"
)

func main() {
	// Step1: Establish Env -- connection connections, logger, etc.
	pg := pg.Connect()
	sqlq := sqlc.New(pg)
	lh := logharbour.New()

	// Step 2: Pass the Env to the handler functions to interact with database
	voucherHandler := voucher.NewHandler(sqlq, lh)
	userHandler := user.NewHandler(sqlq, lh)

	// Step 3: Set up gin router
	r := gin.Default()
	logger := &wscutils.CustomLogger{}
	r.Use(wscutils.CustomLoggerMiddleware(logger))

	// Step 4: Register api handlers
	voucherHandler.RegisterVoucherHandlers(r)
	userHandler.RegisterUserHandlers(r)

	r.Run(":8080")
}
