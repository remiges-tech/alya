package main

import (
	"github.com/gin-gonic/gin"
	"go-framework/internal/webservices/user"
)

func main() {
	router := gin.Default()

	user.RegisterUserHandlers(router)

	router.Run(":8080")
}
