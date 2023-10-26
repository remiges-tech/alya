package rigel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"go-framework/internal/pg/sqlc-gen"
	"go-framework/internal/wscutils"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/sqlc-dev/pqtype"
)

type CreateSchemaRequest struct {
	Name        string               `json:"name" binding:"required"`
	Description *string              `json:"description"`
	Fields      []Field              `json:"fields"  validate:"required,dive"`
	Tags        *[]map[string]string `json:"tags"`
}

type Field struct {
	Name        string `json:"name" validate:"required,min=2,max=50"`
	Type        string `json:"type" validate:"required,oneof=string int float bool"`
	Description string `json:"description" validate:"required,min=2,max=150"`
}

type SchemaResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Fields      []Field   `json:"fields"`
	Tags        []Tag     `json:"tags,omitempty"`
	CreatedBy   string    `json:"created_by"`
	UpdatedBy   string    `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *RigelHandler) createSchema(c *gin.Context) {
	//h.lh.Log("info", "createSchema called")
	// TODO: remove the following line. The value will be set by the auth middleware.
	c.Set("RequestUser", "test")
	requestUser, err := wscutils.GetRequestUser(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, wscutils.NewErrorResponse(err.Error(), err.Error()))
		return
	}

	var schema CreateSchemaRequest

	// step 1: bind request body to struct
	err = wscutils.BindJson(c, &schema)
	if err != nil {
		c.JSON(http.StatusBadRequest, wscutils.NewErrorResponse("invalid_req_body", "invalid_req_body"))
		return
	}

	// step 2: validate request body
	validationErrors := h.validateCreateSchema(schema, c)
	if len(validationErrors) > 0 {
		//h.lh.Log("error", "validation error", validationErrors)
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, validationErrors))
		return
	}

	// step 3: process request
	// convert schema to sqlc.CreateSchemaParams
	fieldsJson, err := json.Marshal(schema.Fields)
	if err != nil {
		log.Printf("error marshalling fields: %v", err)
	}

	tagsJson, err := json.Marshal(schema.Tags)
	if err != nil {
		log.Printf("Error marshalling tags: %v", err)
	}

	// Create the parameters for the CreateSchema function
	createSchemaParams := sqlc.CreateSchemaParams{
		Name: schema.Name,
		Description: sql.NullString{
			String: func() string {
				if schema.Description != nil {
					return *schema.Description
				}
				return ""
			}(),
			Valid: schema.Description != nil,
		},
		Fields: fieldsJson,
		Tags: pqtype.NullRawMessage{
			RawMessage: tagsJson,
			Valid:      len(tagsJson) > 0,
		},
		CreatedBy: requestUser,
		UpdatedBy: requestUser,
	}

	// Call the CreateSchema function
	ctx := context.TODO() // perhaps we should create one from the gin context
	newSchema, err := h.sqlq.CreateSchema(ctx, createSchemaParams)
	if err != nil {
		fmt.Println("error creating schema version", err)
		c.JSON(http.StatusInternalServerError, wscutils.NewErrorResponse("internal_error", "internal_error"))
		return
	}

	// Convert newSchema to SchemaResponse
	schemaResponse := ConvertToSchemaResponse(newSchema)

	// step 4: send response
	c.JSON(http.StatusOK, wscutils.NewSuccessResponse(schemaResponse))
}

func (h *RigelHandler) validateCreateSchema(schemaVersion CreateSchemaRequest, c *gin.Context) []wscutils.ErrorMessage {
	var validationErrors []wscutils.ErrorMessage

	// step 2.1: validate request body
	validationErrors = wscutils.WscValidate(schemaVersion, schemaVersion.getValsForCreateSchemaVersionError)

	// if there are standard validation errors, return
	// do not execute custom validations
	if len(validationErrors) > 0 {
		return validationErrors
	}

	return validationErrors

}

func (sv *CreateSchemaRequest) getValsForCreateSchemaVersionError(err validator.FieldError) []string {
	var vals []string
	errField := err.Field()
	fmt.Println("errField", errField)
	switch err.Field() {
	case "Type":
		switch err.Tag() {
		case "oneof":
			vals = append(vals, "string, int, float, bool")
			vals = append(vals, err.Value().(string))
		}

	}
	return vals

}

func ConvertToSchemaResponse(schema sqlc.Schema) SchemaResponse {
	var fields []Field
	if err := json.Unmarshal(schema.Fields, &fields); err != nil {
		log.Printf("Error unmarshalling fields: %v", err)
	}
	var tags []Tag
	if err := json.Unmarshal(schema.Tags.RawMessage, &tags); err != nil {
		log.Printf("Error unmarshalling fields: %v", err)
	}
	return SchemaResponse{
		ID:          int64(schema.ID),
		Name:        schema.Name,
		Tags:        tags,
		Description: schema.Description.String,
		Fields:      fields,
		CreatedBy:   schema.CreatedBy,
		UpdatedBy:   schema.UpdatedBy,
		CreatedAt:   schema.CreatedAt.Time,
		UpdatedAt:   schema.UpdatedAt.Time,
	}
}
