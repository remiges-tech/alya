package transport

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/api"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/app"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/repository"
	"github.com/remiges-tech/alya/restutils"
	"github.com/remiges-tech/alya/service"
	"github.com/remiges-tech/alya/wscutils"
)

type OrderHandler struct {
	app       *app.OrderService
	validator *restutils.Validator
}

func NewOrderHandler(appService *app.OrderService, validator *restutils.Validator) *OrderHandler {
	return &OrderHandler{app: appService, validator: validator}
}

func (h *OrderHandler) CreateOrder(c *gin.Context, _ *service.Service) {
	var req api.CreateOrderRequest
	if err := restutils.BindBody(c, &req); err != nil {
		restutils.WriteProblem(c, restutils.ProblemFromBindError(err))
		return
	}
	if errs := h.validator.Validate(req); len(errs) > 0 {
		restutils.WriteProblem(c, restutils.ValidationProblem(errs))
		return
	}

	order, err := h.app.CreateOrder(c.Request.Context(), req)
	if err != nil {
		restutils.WriteProblem(c, problemFromOrderAppError(err))
		return
	}
	location := fmt.Sprintf("/orders/%d", order.ID)
	restutils.WriteCreated(c, location, toOrderResponse(order))
}

func (h *OrderHandler) ListOrders(c *gin.Context, _ *service.Service) {
	orders, err := h.app.ListOrders(c.Request.Context())
	if err != nil {
		restutils.WriteProblem(c, restutils.InternalServerError())
		return
	}
	response := make([]api.OrderResponse, 0, len(orders))
	for _, order := range orders {
		response = append(response, toOrderResponse(order))
	}
	restutils.WriteOK(c, response)
}

func (h *OrderHandler) GetOrder(c *gin.Context, _ *service.Service) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		restutils.WriteProblem(c, badRequestProblem("invalid order id"))
		return
	}
	order, err := h.app.GetOrder(c.Request.Context(), id)
	if err != nil {
		restutils.WriteProblem(c, problemFromOrderAppError(err))
		return
	}
	restutils.WriteOK(c, toOrderResponse(order))
}

func (h *OrderHandler) UpdateOrder(c *gin.Context, _ *service.Service) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		restutils.WriteProblem(c, badRequestProblem("invalid order id"))
		return
	}

	var req api.UpdateOrderRequest
	if err := restutils.BindBody(c, &req); err != nil {
		restutils.WriteProblem(c, restutils.ProblemFromBindError(err))
		return
	}
	if errs := validateUpdateOrderRequest(req, h.validator); len(errs) > 0 {
		restutils.WriteProblem(c, restutils.ValidationProblem(errs))
		return
	}

	order, err := h.app.UpdateOrder(c.Request.Context(), id, req)
	if err != nil {
		restutils.WriteProblem(c, problemFromOrderAppError(err))
		return
	}
	restutils.WriteOK(c, toOrderResponse(order))
}

func (h *OrderHandler) DeleteOrder(c *gin.Context, _ *service.Service) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		restutils.WriteProblem(c, badRequestProblem("invalid order id"))
		return
	}
	if err := h.app.DeleteOrder(c.Request.Context(), id); err != nil {
		restutils.WriteProblem(c, problemFromOrderAppError(err))
		return
	}
	restutils.WriteNoContent(c)
}

func validateUpdateOrderRequest(req api.UpdateOrderRequest, validator *restutils.Validator) []restutils.FieldError {
	var fieldErrors []restutils.FieldError

	if req.UserID.Null {
		fieldErrors = append(fieldErrors, restutils.FieldError{ErrorMessage: wscutils.BuildErrorMessage(104, "invalid", "user_id"), Message: "must not be null"})
	}
	if req.Number.Null {
		fieldErrors = append(fieldErrors, restutils.FieldError{ErrorMessage: wscutils.BuildErrorMessage(104, "invalid", "number"), Message: "must not be null"})
	}
	if req.Status.Null {
		fieldErrors = append(fieldErrors, restutils.FieldError{ErrorMessage: wscutils.BuildErrorMessage(104, "invalid", "status"), Message: "must not be null"})
	}
	if req.TotalAmount.Null {
		fieldErrors = append(fieldErrors, restutils.FieldError{ErrorMessage: wscutils.BuildErrorMessage(104, "invalid", "total_amount"), Message: "must not be null"})
	}
	if value, ok := req.UserID.Get(); ok {
		tmp := struct {
			UserID int64 `json:"user_id" validate:"gte=1"`
		}{UserID: value}
		fieldErrors = append(fieldErrors, validator.Validate(tmp)...)
	}
	if value, ok := req.Number.Get(); ok {
		tmp := struct {
			Number string `json:"number" validate:"min=3,max=30,alphanum"`
		}{Number: value}
		fieldErrors = append(fieldErrors, validator.Validate(tmp)...)
	}
	if value, ok := req.Status.Get(); ok {
		tmp := struct {
			Status string `json:"status" validate:"oneof=pending paid cancelled"`
		}{Status: value}
		fieldErrors = append(fieldErrors, validator.Validate(tmp)...)
	}
	if value, ok := req.TotalAmount.Get(); ok {
		tmp := struct {
			TotalAmount int64 `json:"total_amount" validate:"gte=0"`
		}{TotalAmount: value}
		fieldErrors = append(fieldErrors, validator.Validate(tmp)...)
	}
	return fieldErrors
}

func problemFromOrderAppError(err error) restutils.Problem {
	switch {
	case errors.Is(err, app.ErrOrderNotFound):
		return restutils.NewProblem(http.StatusNotFound, "https://alya.dev/problems/not-found", "Resource not found", "order was not found")
	case errors.Is(err, app.ErrUserNotFound):
		return restutils.NewProblem(http.StatusUnprocessableEntity, "https://alya.dev/problems/validation", "Validation failed", "user_id does not reference an existing user")
	case errors.Is(err, app.ErrOrderNumberUsed):
		return restutils.NewProblem(http.StatusConflict, "https://alya.dev/problems/conflict", "Conflict", "order number already exists")
	default:
		return restutils.InternalServerError()
	}
}

func toOrderResponse(order repository.Order) api.OrderResponse {
	return api.OrderResponse{
		ID:          order.ID,
		UserID:      order.UserID,
		Number:      order.Number,
		Status:      order.Status,
		TotalAmount: order.TotalAmount,
	}
}
