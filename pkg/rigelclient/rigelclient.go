package rigelclient

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

type ConfigClient struct {
	BaseURL string
	Client  *resty.Client
}

type ConfigResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	SchemaID    int32     `json:"schema_id"`
	Active      bool      `json:"active"`
	Description string    `json:"description,omitempty"`
	Tags        []Tag     `json:"tags,omitempty"`
	Values      []Value   `json:"values"`
	CreatedBy   string    `json:"created_by"`
	UpdatedBy   string    `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Value struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type Tag struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type Response struct {
	Status   string         `json:"status"`
	Data     ConfigResponse `json:"data"`
	Messages []ErrorMessage `json:"messages"`
}

type ErrorMessage struct {
	Field   string   `json:"field"`
	Code    string   `json:"code"`
	Msgcode int      `json:"msgcode"`
	Vals    []string `json:"vals,omitempty"` // omit if Vals is empty
}

func (c *ConfigClient) LoadConfig(configName string, schemaName string, v interface{}) error {
	resp, err := c.Client.R().
		SetResult(&Response{}).
		Get(c.BaseURL + "/config?config_name=" + configName + "&schema_name=" + schemaName)

	if err != nil {
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("error from server: %v", resp.Status())
	}

	response := resp.Result().(*Response)

	// Convert the Values slice to a map
	valuesMap := make(map[string]interface{})
	for _, value := range response.Data.Values {
		valuesMap[value.Key] = value.Value
	}

	// Convert the values map to a JSON string
	valuesJson, err := json.Marshal(valuesMap)
	if err != nil {
		return err
	}

	// Unmarshal the values JSON into the provided struct
	err = json.Unmarshal(valuesJson, v)
	if err != nil {
		return err
	}

	return nil
}
