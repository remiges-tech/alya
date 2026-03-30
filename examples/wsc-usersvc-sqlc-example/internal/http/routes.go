package transport

import "github.com/gin-gonic/gin"

func RegisterRoutes(router gin.IRoutes, userHandler *UserHandler, orderHandler *OrderHandler) {
	router.POST("/users", userHandler.CreateUser)
	router.GET("/users", userHandler.ListUsers)
	router.GET("/users/:id", userHandler.GetUser)
	router.PATCH("/users/:id", userHandler.UpdateUser)
	router.DELETE("/users/:id", userHandler.DeleteUser)

	router.POST("/orders", orderHandler.CreateOrder)
	router.GET("/orders", orderHandler.ListOrders)
	router.GET("/orders/:id", orderHandler.GetOrder)
	router.PATCH("/orders/:id", orderHandler.UpdateOrder)
	router.DELETE("/orders/:id", orderHandler.DeleteOrder)
}
