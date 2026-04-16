package transport

import (
	"net/http"

	"github.com/remiges-tech/alya/service"
)

func RegisterRoutes(s *service.Service, userHandler *UserHandler, orderHandler *OrderHandler) {
	s.RegisterRoute(http.MethodPost, "/users", userHandler.CreateUser)
	s.RegisterRoute(http.MethodGet, "/users", userHandler.ListUsers)
	s.RegisterRoute(http.MethodGet, "/users/:id", userHandler.GetUser)
	s.RegisterRoute(http.MethodPatch, "/users/:id", userHandler.UpdateUser)
	s.RegisterRoute(http.MethodDelete, "/users/:id", userHandler.DeleteUser)

	s.RegisterRoute(http.MethodPost, "/orders", orderHandler.CreateOrder)
	s.RegisterRoute(http.MethodGet, "/orders", orderHandler.ListOrders)
	s.RegisterRoute(http.MethodGet, "/orders/:id", orderHandler.GetOrder)
	s.RegisterRoute(http.MethodPatch, "/orders/:id", orderHandler.UpdateOrder)
	s.RegisterRoute(http.MethodDelete, "/orders/:id", orderHandler.DeleteOrder)
}
