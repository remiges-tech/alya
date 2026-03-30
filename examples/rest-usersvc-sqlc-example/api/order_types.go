package api

import "github.com/remiges-tech/alya/wscutils"

type CreateOrderRequest struct {
	UserID      int64  `json:"user_id" validate:"required,gte=1"`
	Number      string `json:"number" validate:"required,min=3,max=30,alphanum"`
	Status      string `json:"status" validate:"required,oneof=pending paid cancelled"`
	TotalAmount int64  `json:"total_amount" validate:"gte=0"`
}

type UpdateOrderRequest struct {
	UserID      wscutils.Optional[int64]  `json:"user_id"`
	Number      wscutils.Optional[string] `json:"number"`
	Status      wscutils.Optional[string] `json:"status"`
	TotalAmount wscutils.Optional[int64]  `json:"total_amount"`
}

type OrderResponse struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Number      string `json:"number"`
	Status      string `json:"status"`
	TotalAmount int64  `json:"total_amount"`
}
