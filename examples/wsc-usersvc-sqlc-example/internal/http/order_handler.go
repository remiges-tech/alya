package transport

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/api"
	"github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/repository"
	"github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/service"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
)

type OrderHandler struct {
	app       *app.OrderService
	validator *wscutils.Validator
	logger    *logharbour.Logger
}

func NewOrderHandler(appService *app.OrderService, validator *wscutils.Validator, logger *logharbour.Logger) *OrderHandler {
	return &OrderHandler{app: appService, validator: validator, logger: logger}
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	h.logger.Info().LogActivity("CreateOrder request received", nil)

	var req api.CreateOrderRequest
	if err := wscutils.BindData(c, &req); err != nil {
		wscutils.SendError(c, http.StatusBadRequest, invalidJSONMessages())
		return
	}
	if errs := h.validator.Validate(req); len(errs) > 0 {
		wscutils.SendError(c, http.StatusBadRequest, errs)
		return
	}

	order, err := h.app.CreateOrder(c.Request.Context(), req)
	if err != nil {
		status, messages := h.orderAppError(err)
		wscutils.SendError(c, status, messages)
		return
	}
	wscutils.SendCreated(c, "/orders/"+int64ToString(order.ID), toOrderResponse(order))
}

func (h *OrderHandler) ListOrders(c *gin.Context) {
	h.logger.Info().LogActivity("ListOrders request received", nil)

	orders, err := h.app.ListOrders(c.Request.Context())
	if err != nil {
		h.logger.Error(fmt.Errorf("list orders failed: %w", err)).LogActivity("Request failed", nil)
		wscutils.SendError(c, http.StatusInternalServerError, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "internal", "")})
		return
	}
	response := make([]api.OrderResponse, 0, len(orders))
	for _, order := range orders {
		response = append(response, toOrderResponse(order))
	}
	wscutils.SendOK(c, response)
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	h.logger.Info().LogActivity("GetOrder request received", map[string]any{"id": c.Param("id")})

	id, err := wscutils.ParseInt64PathParam(c, "id")
	if err != nil {
		wscutils.SendError(c, http.StatusBadRequest, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "id")})
		return
	}
	order, err := h.app.GetOrder(c.Request.Context(), id)
	if err != nil {
		status, messages := h.orderAppError(err)
		wscutils.SendError(c, status, messages)
		return
	}
	wscutils.SendOK(c, toOrderResponse(order))
}

func (h *OrderHandler) UpdateOrder(c *gin.Context) {
	h.logger.Info().LogActivity("UpdateOrder request received", map[string]any{"id": c.Param("id")})

	id, err := wscutils.ParseInt64PathParam(c, "id")
	if err != nil {
		wscutils.SendError(c, http.StatusBadRequest, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "id")})
		return
	}

	var req api.UpdateOrderRequest
	if err := wscutils.BindData(c, &req); err != nil {
		wscutils.SendError(c, http.StatusBadRequest, invalidJSONMessages())
		return
	}
	if errs := validateUpdateOrderRequest(req, h.validator); len(errs) > 0 {
		wscutils.SendError(c, http.StatusBadRequest, errs)
		return
	}

	order, err := h.app.UpdateOrder(c.Request.Context(), id, req)
	if err != nil {
		status, messages := h.orderAppError(err)
		wscutils.SendError(c, status, messages)
		return
	}
	wscutils.SendOK(c, toOrderResponse(order))
}

func (h *OrderHandler) DeleteOrder(c *gin.Context) {
	h.logger.Info().LogActivity("DeleteOrder request received", map[string]any{"id": c.Param("id")})

	id, err := wscutils.ParseInt64PathParam(c, "id")
	if err != nil {
		wscutils.SendError(c, http.StatusBadRequest, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "id")})
		return
	}
	if err := h.app.DeleteOrder(c.Request.Context(), id); err != nil {
		status, messages := h.orderAppError(err)
		wscutils.SendError(c, status, messages)
		return
	}
	wscutils.SendDeleted(c)
}

func validateUpdateOrderRequest(req api.UpdateOrderRequest, validator *wscutils.Validator) []wscutils.ErrorMessage {
	var errs []wscutils.ErrorMessage
	if req.UserID.Null {
		errs = append(errs, wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "user_id"))
	}
	if req.Number.Null {
		errs = append(errs, wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "number"))
	}
	if req.Status.Null {
		errs = append(errs, wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "status"))
	}
	if req.TotalAmount.Null {
		errs = append(errs, wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "total_amount"))
	}
	if value, ok := req.UserID.Get(); ok {
		tmp := struct {
			UserID int64 `json:"user_id" validate:"gte=1"`
		}{UserID: value}
		errs = append(errs, validator.Validate(tmp)...)
	}
	if value, ok := req.Number.Get(); ok {
		tmp := struct {
			Number string `json:"number" validate:"min=3,max=30,alphanum"`
		}{Number: value}
		errs = append(errs, validator.Validate(tmp)...)
	}
	if value, ok := req.Status.Get(); ok {
		tmp := struct {
			Status string `json:"status" validate:"oneof=pending paid cancelled"`
		}{Status: value}
		errs = append(errs, validator.Validate(tmp)...)
	}
	if value, ok := req.TotalAmount.Get(); ok {
		tmp := struct {
			TotalAmount int64 `json:"total_amount" validate:"gte=0"`
		}{TotalAmount: value}
		errs = append(errs, validator.Validate(tmp)...)
	}
	return errs
}

func (h *OrderHandler) orderAppError(err error) (int, []wscutils.ErrorMessage) {
	switch {
	case errors.Is(err, app.ErrOrderNotFound):
		return http.StatusNotFound, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDNotFound, "missing", "id")}
	case errors.Is(err, app.ErrUserNotFound):
		return http.StatusBadRequest, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "user_id")}
	case errors.Is(err, app.ErrOrderNumberUsed):
		return http.StatusConflict, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDConflict, "exists", "number")}
	default:
		h.logger.Error(fmt.Errorf("order request failed: %w", err)).LogActivity("Request failed", nil)
		return http.StatusInternalServerError, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "internal", "")}
	}
}

func toOrderResponse(order repository.Order) api.OrderResponse {
	return api.OrderResponse{ID: order.ID, UserID: order.UserID, Number: order.Number, Status: order.Status, TotalAmount: order.TotalAmount}
}
