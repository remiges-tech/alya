package main

import (
	"github.com/gin-gonic/gin"
	"go-framework/internal/infra"
	"go-framework/internal/webservices/rigel"
	"go-framework/internal/webservices/user"
	voucher "go-framework/internal/webservices/vouchers"
)

// Define your application's config struct
type AppConfig struct {
	DatabaseURL string `json:"database_url"`
	Port        int    `json:"port"`
}

func main() {
	//sqlq, lh, rdb := infra.InitInfraServices()
	sqlq, lh, _ := infra.InitInfraServices()
	//r := infra.SetupRouter(lh, rdb)
	r := gin.Default()

	// Pass the Env to the handler functions to interact with database
	voucherHandler := voucher.NewHandler(sqlq, lh)
	userHandler := user.NewHandler(sqlq, lh)
	rigelHandler := rigel.NewHandler(sqlq, lh)

	// Register api handlers
	voucherHandler.RegisterVoucherHandlers(r)
	userHandler.RegisterUserHandlers(r)
	rigelHandler.RegisterHandlers(r)

	r.Run(":8080")
}
