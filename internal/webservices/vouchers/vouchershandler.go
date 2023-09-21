package voucher

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"go-framework/internal/pg/sqlc-gen"
	"go-framework/internal/wscutils"
	"go-framework/logharbour"
	"net/http"
	"strconv"
	"time"
)

type VoucherHandler struct {
	sqlq *sqlc.Queries
	lh   *logharbour.LogHarbour
}

func NewHandler(sqlq *sqlc.Queries, lh *logharbour.LogHarbour) *VoucherHandler {
	return &VoucherHandler{
		sqlq: sqlq,
		lh:   lh,
	}
}

func (h *VoucherHandler) RegisterVoucherHandlers(router *gin.Engine) {
	router.POST("/voucher", h.createVoucher)
	router.GET("/voucher/:voucher_id", h.getVoucher)
	router.PUT("/voucher/:voucher_id", h.updateVoucher)
	router.DELETE("/voucher/:voucher_id", h.deleteVoucher)
}

type Voucher struct {
	EmployeeID  int32   `json:"employee_id" validate:"required"`
	DateOfClaim string  `json:"date_of_claim" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	Amount      float64 `json:"amount" validate:"required,min=0"`
	Description *string `json:"description"`
}

func (h *VoucherHandler) createVoucher(c *gin.Context) {
	h.lh.Log("info", "create voucher called")
	var voucher Voucher
	var createVoucherParams sqlc.CreateVoucherParams

	// step 1: bind request body to struct
	err := wscutils.BindJson(c, &voucher)
	if err != nil {
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, []wscutils.ErrorMessage{wscutils.BuildValidationError("invalid_req_body", "invalida_req_body")}))
		return
	}

	// step 2: validate request body
	validationErrors := validate(voucher)

	// step 3: if there are validation errors, add them to response and send it
	if len(validationErrors) > 0 {
		h.lh.Log("error", "validation error", validationErrors)
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, validationErrors))
		return
	}

	// step 4: process the request

	// Convert DateOfClaim to time.Time as required by CreateVoucher function
	timeOfClaim, _ := time.Parse(time.RFC3339, voucher.DateOfClaim)

	// Create and initialize createVoucherParams with data from the voucher struct
	createVoucherParams = sqlc.CreateVoucherParams{
		EmployeeID:  voucher.EmployeeID,
		DateOfClaim: timeOfClaim,
		Amount:      voucher.Amount,
		Description: sql.NullString{
			String: *voucher.Description,
			Valid:  voucher.Description != nil,
		},
	}

	// Call the SQLC generated function to insert the voucher
	_, err = h.sqlq.CreateVoucher(c, createVoucherParams)
	if err != nil {
		// log the error
		h.lh.Log("error", "error creating voucher", err.Error())
		// buildvalidationerror for something went wrong
		validationError := wscutils.BuildValidationError("unknown", "unknown")
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, []wscutils.ErrorMessage{validationError}))
		return
	}

	// step 5: if there are no errors, send success response
	c.JSON(http.StatusOK, wscutils.NewResponse(wscutils.SuccessStatus, &voucher, []wscutils.ErrorMessage{}))
}

func (h *VoucherHandler) getVoucher(c *gin.Context) {
	voucherID := c.Param("voucher_id")

	// Call the SQLC generated function to get the voucher
	voucherIDInt, err := strconv.Atoi(voucherID)
	if err != nil {
		// buildvalidationerror for invalid voucher id
		validationError := wscutils.BuildValidationError("voucher_id", "invalid_voucher_id")
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, []wscutils.ErrorMessage{validationError}))
		return
	}
	voucher, err := h.sqlq.GetVoucher(c, int32(voucherIDInt))

	// Check the error and respond accordingly

	c.JSON(http.StatusOK, wscutils.NewResponse(wscutils.SuccessStatus, &voucher, []wscutils.ErrorMessage{}))
}

func (h *VoucherHandler) updateVoucher(c *gin.Context) {
	voucherID := c.Param("voucher_id")
	var voucher sqlc.UpdateVoucherParams

	// Step 1: bind request body to UpdateVoucherParams struct
	if err := wscutils.BindJson(c, &voucher); err != nil {
		return
	}

	// Convert voucherID to int32 as required by UpdateVoucher function
	voucherIDInt, err := strconv.Atoi(voucherID)
	if err != nil {
		// Build validation error for invalid voucher id
		validationError := wscutils.BuildValidationError("voucher_id", "invalid_voucher_id")
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, []wscutils.ErrorMessage{validationError}))
		return
	}
	// Set converted voucherIDInt to voucher.VoucherID
	voucher.VoucherID = int32(voucherIDInt)

	// Step 2: Call the SQLC generated function to update the voucher
	updatedVoucher, err := h.sqlq.UpdateVoucher(c, voucher)

	// Check the error and respond accordingly
	// your error handling code here

	// Step 3: Respond with the updated voucher details
	c.JSON(http.StatusOK, wscutils.NewResponse(wscutils.SuccessStatus, &updatedVoucher, []wscutils.ErrorMessage{}))
}

func (h *VoucherHandler) deleteVoucher(c *gin.Context) {
	voucherID := c.Param("voucher_id")

	// Convert voucherID to int32 as required by DeleteVoucher function
	voucherIDInt, err := strconv.Atoi(voucherID)
	if err != nil {
		// Build validation error for invalid voucher id
		validationError := wscutils.BuildValidationError("voucher_id", "invalid_voucher_id")
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, []wscutils.ErrorMessage{validationError}))
		return
	}

	// Call the SQLC generated function to delete the voucher
	err = h.sqlq.DeleteVoucher(c, int32(voucherIDInt))

	// Check the error and respond accordingly
	// your error handling code here

	// Respond with the success message
	c.JSON(http.StatusOK, wscutils.NewResponse(wscutils.SuccessStatus, "Voucher deleted successfully", []wscutils.ErrorMessage{}))
}

func validate(voucher Voucher) []wscutils.ErrorMessage {
	validationErrors := wscutils.WscValidate(voucher)

	return validationErrors
}
