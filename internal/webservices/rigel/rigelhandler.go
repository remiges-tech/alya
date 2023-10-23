package rigel

import (
	"database/sql"
	"encoding/json"
	"go-framework/internal/pg/sqlc-gen"
	"go-framework/internal/wscutils"
	"go-framework/logharbour"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/sqlc-dev/pqtype"
)

type RigelHandler struct {
	sqlq *sqlc.Queries
	lh   *logharbour.LogHarbour
}

func NewHandler(sqlq *sqlc.Queries, lh *logharbour.LogHarbour) *RigelHandler {
	return &RigelHandler{
		sqlq: sqlq,
		lh:   lh,
	}
}

func (h *RigelHandler) RegisterHandlers(router *gin.Engine) {
	router.POST("/rigel/schema", h.createSchema)
}

type Schema struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Tags        []map[string]string `json:"tags"`
	Active      bool                `json:"active"`
	CreatedBy   string              `json:"created_by"`
	UpdatedBy   string              `json:"updated_by"`
	Fields      []Field             `json:"fields"`
}

type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

func (h *RigelHandler) createSchema(c *gin.Context) {
	h.lh.Log("info", "createSchema called")
	var schema Schema
	var createSchemaParams sqlc.CreateSchemaParams

	// step 1: bind request body to struct
	err := wscutils.BindJson(c, &schema)

	if err != nil {
		c.JSON(http.StatusBadRequest, wscutils.NewErrorResponse("invalid_req_body", "invalid_req_body"))
		return
	}

	// step 2: validate request body
	validationErrors := validate(schema)

	// step 3: if there are validation errors, add them to response and send it
	if len(validationErrors) > 0 {
		h.lh.Log("error", "validation error", validationErrors)
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, validationErrors))
		return
	}

	// step 4: process the request

	tagsJson, err := json.Marshal(schema.Tags)
	if err != nil {
		log.Printf("Error marshalling tags: %v", err)
	}

	// Create and initialize createSchemaParams with data from the voucher struct
	createSchemaParams = sqlc.CreateSchemaParams{
		Name: schema.Name,
		Description: sql.NullString{
			String: schema.Description,
			Valid:  schema.Description != "",
		},
		Tags: pqtype.NullRawMessage{
			RawMessage: tagsJson,
			Valid:      len(schema.Tags) > 0,
		},
		Active: sql.NullBool{
			Bool:  schema.Active,
			Valid: true,
		},
		CreatedBy: sql.NullString{
			String: schema.CreatedBy,
			Valid:  schema.CreatedBy != "",
		},
		UpdatedBy: sql.NullString{
			String: schema.UpdatedBy,
			Valid:  schema.UpdatedBy != "",
		},
	}

	// Call the SQLC generated function to insert the voucher
	_, err = h.sqlq.CreateSchema(c, createSchemaParams)
	if err != nil {
		// log the error
		h.lh.Log("error", "error creating voucher", err.Error())
		// buildvalidationerror for something went wrong
		c.JSON(http.StatusInternalServerError, wscutils.NewErrorResponse("database_error", "error_creating_voucher"))

		return
	}

	// step 5: if there are no errors, send success response
	c.JSON(http.StatusOK, wscutils.NewResponse(wscutils.SuccessStatus, &schema, []wscutils.ErrorMessage{}))
}

// validate validates the request body
func validate(schema Schema) []wscutils.ErrorMessage {
	// step 2.1: validate request body using standard validator
	validationErrors := wscutils.WscValidate(schema, schema.getValsForSchemaCreateError)

	// step 2.2: add request-specific vals to validation errors
	// NOTE: it mutates validationErrors
	validationErrors = addVals(validationErrors, schema)

	// if there are standard validation errors, return
	// do not execute custom validations
	if len(validationErrors) > 0 {
		return validationErrors
	}

	// step 2.3: check request specific custom validations and add errors
	validationErrors = addCustomValidationErrors(validationErrors, schema)

	return validationErrors
}

func (s Schema) getValsForSchemaCreateError(err validator.FieldError) []string {
	switch err.Field() {
	case "Name":
		return []string{"name1", "name2"}
	case "Email":
		return []string{"email1", "email2"}
	default:
		return []string{}
	}
}

func addCustomValidationErrors(validationErrors []wscutils.ErrorMessage, schema Schema) []wscutils.ErrorMessage {
	return nil
}

func addVals(validationErrors []wscutils.ErrorMessage, schema Schema) []wscutils.ErrorMessage {
	return nil
}
