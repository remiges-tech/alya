package main

import (
	"github.com/gin-gonic/gin"
	"go-framework/internal/webservices/user"
	"go-framework/internal/wscutils"
)

func main() {
	r := gin.Default()
	logger := &wscutils.CustomLogger{}

	r.Use(wscutils.CustomLoggerMiddleware(logger))

	user.RegisterUserHandlers(r)

	r.Run(":8080")
}
