//go:build go1.24

package wscutils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestOptionalOmitZero tests the omitzero tag behavior in Go 1.24+
func TestOptionalOmitZero(t *testing.T) {
	type TestStruct struct {
		Name     string           `json:"name,omitzero"`
		Email    Optional[string] `json:"email,omitzero"`
		Age      Optional[int]    `json:"age,omitzero"`
		IsActive Optional[bool]   `json:"isActive,omitzero"`
	}

	tests := []struct {
		name     string
		input    TestStruct
		expected string
	}{
		{
			name: "All optional fields absent",
			input: TestStruct{
				Name:     "Arjun",
				Email:    NewOptionalAbsent[string](),
				Age:      NewOptionalAbsent[int](),
				IsActive: NewOptionalAbsent[bool](),
			},
			expected: `{"name":"Arjun"}`,
		},
		{
			name: "Mix of present and absent",
			input: TestStruct{
				Name:     "Priya",
				Email:    NewOptional("priya@example.com"),
				Age:      NewOptionalAbsent[int](),
				IsActive: NewOptional(true),
			},
			expected: `{"name":"Priya","email":"priya@example.com","isActive":true}`,
		},
		{
			name: "Null values included",
			input: TestStruct{
				Name:     "Rahul",
				Email:    NewOptionalNull[string](),
				Age:      NewOptional(25),
				IsActive: NewOptionalAbsent[bool](),
			},
			expected: `{"name":"Rahul","email":null,"age":25}`,
		},
		{
			name: "Zero values but present",
			input: TestStruct{
				Name:     "", // Empty string omitted
				Email:    NewOptional(""), // Empty but present
				Age:      NewOptional(0),  // Zero but present
				IsActive: NewOptional(false), // False but present
			},
			expected: `{"email":"","age":0,"isActive":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

// TestOptionalOmitZeroComplex tests omitzero with complex types
func TestOptionalOmitZeroComplex(t *testing.T) {
	type Address struct {
		Street string `json:"street,omitzero"`
		City   string `json:"city,omitzero"`
	}

	type ComplexStruct struct {
		Tags     Optional[[]string]       `json:"tags,omitzero"`
		Address  Optional[Address]        `json:"address,omitzero"`
		Metadata Optional[map[string]any] `json:"metadata,omitzero"`
	}

	tests := []struct {
		name     string
		input    ComplexStruct
		expected string
	}{
		{
			name: "All absent",
			input: ComplexStruct{
				Tags:     NewOptionalAbsent[[]string](),
				Address:  NewOptionalAbsent[Address](),
				Metadata: NewOptionalAbsent[map[string]any](),
			},
			expected: `{}`,
		},
		{
			name: "Complex types present",
			input: ComplexStruct{
				Tags:    NewOptional([]string{"tag1", "tag2"}),
				Address: NewOptional(Address{Street: "MG Road", City: "Bangalore"}),
				Metadata: NewOptionalAbsent[map[string]any](),
			},
			expected: `{"tags":["tag1","tag2"],"address":{"street":"MG Road","city":"Bangalore"}}`,
		},
		{
			name: "Null complex types",
			input: ComplexStruct{
				Tags:     NewOptionalNull[[]string](),
				Address:  NewOptionalNull[Address](),
				Metadata: NewOptional(map[string]any{"key": "value"}),
			},
			expected: `{"tags":null,"address":null,"metadata":{"key":"value"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}