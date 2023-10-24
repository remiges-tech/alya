package rigel

import (
	"database/sql"
	"encoding/json"
	"go-framework/internal/pg/sqlc-gen"
	"go-framework/internal/wscutils"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/sqlc-dev/pqtype"
)

type Config struct {
	Name        string  `'json:"name" binding:"required" validate:"required,min=2,max=150"`
	Description *string `validate:"omitempty,min=2,max=150"`
	Active      *bool   `validate:"omitempty"`
	Tags        *[]Tag  `validate:"omitempty"`
}

type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (h *RigelHandler) createConfig(c *gin.Context) {
	h.lh.Log("info", "createConfig called")
	var config Config
	var createConfigParams sqlc.CreateConfigParams

	// Get the RequestUser from the gin context
	requestUserStr, err := wscutils.GetRequestUser(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, wscutils.NewErrorResponse(err.Error(), err.Error()))
		return
	}

	// step 1: bind request body to struct
	err = wscutils.BindJson(c, &config)

	if err != nil {
		c.JSON(http.StatusBadRequest, wscutils.NewErrorResponse("invalid_req_body", "invalid_req_body"))
		return
	}

	// step 2: validate request body
	validationErrors := validateConfigCreate(config)

	// step 3: if there are validation errors, add them to response and send it
	if len(validationErrors) > 0 {
		h.lh.Log("error", "validation error", validationErrors)
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, validationErrors))
		return
	}

	// step 4: process the request

	tagsJson, err := json.Marshal(config.Tags)
	if err != nil {
		log.Printf("Error marshalling tags: %v", err)
	}

	// Create and initialize createConfigParams with data from the voucher struct
	createConfigParams = sqlc.CreateConfigParams{
		Name: config.Name,
		Description: sql.NullString{
			String: func() string {
				if config.Description != nil {
					return *config.Description
				}
				return ""
			}(),
			Valid: config.Description != nil,
		},
		Tags: pqtype.NullRawMessage{
			RawMessage: tagsJson,
			Valid:      len(tagsJson) > 0,
		},
		Active: sql.NullBool{
			Bool: func() bool {
				if config.Active != nil {
					return *config.Active
				}
				return false
			}(),
			Valid: config.Active != nil,
		},
		CreatedBy: requestUserStr,
		UpdatedBy: requestUserStr,
	}

	// Call the SQLC generated function to insert the voucher
	_, err = h.sqlq.CreateConfig(c, createConfigParams)
	if err != nil {
		// log the error
		h.lh.Log("error", "error creating voucher", err.Error())
		// buildvalidationerror for something went wrong
		c.JSON(http.StatusInternalServerError, wscutils.NewErrorResponse("database_error", "error_creating_voucher"))

		return
	}

	// step 5: if there are no errors, send success response
	c.JSON(http.StatusOK, wscutils.NewResponse(wscutils.SuccessStatus, &config, []wscutils.ErrorMessage{}))
}

func validateConfigCreate(config Config) []wscutils.ErrorMessage {
	// step 2.1: validate request body using standard validator
	validationErrors := wscutils.WscValidate(config, config.getValsForUserError)

	// if there are standard validation errors, return
	// do not execute custom validations
	if len(validationErrors) > 0 {
		return validationErrors
	}

	// step 2.3: check request specific custom validations and add errors
	// we do not need any custom validation for createConfig

	return validationErrors
}

// getValsForUserError returns a slice of strings to be used as vals for a validation error.
// The vals are determined based on the field and the validation rule that failed.
func (c *Config) getValsForUserError(err validator.FieldError) []string {
	var vals []string

	// check fields (err.Field()) and its validation rule (err.Tag()) that failed
	switch err.Field() {
	case "Name":
		switch err.Tag() {
		case "min":
			vals = append(vals, "2")
			vals = append(vals, strconv.Itoa(len(c.Name))) // provided value that failed validation
		case "max":
			vals = append(vals, "150")
			vals = append(vals, strconv.Itoa(len(c.Name))) // provided value that failed validation
		}
		// Add more cases as needed
	}
	return vals
}
