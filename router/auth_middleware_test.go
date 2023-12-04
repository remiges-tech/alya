package router

import (
	"context"
	"fmt"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/mock"
)

type MockTokenCache struct {
	mock.Mock
}

func (r *MockTokenCache) Set(token string) error {
	args := r.Called(token)
	return args.Error(0)
}

func (r *MockTokenCache) Get(token string) (bool, error) {
	args := r.Called(token)
	return args.Bool(0), args.Error(1)
}

type MockTokenVerifier struct {
	mock.Mock
}

func (m *MockTokenVerifier) Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	args := m.Called(ctx, rawIDToken)

	// Check if the first argument can be casted to *oidc.IDToken
	idToken, ok := args.Get(0).(*oidc.IDToken)
	if !ok {
		return nil, fmt.Errorf("unexpected type for first argument")
	}

	// Check if the second argument can be casted to error
	err, ok := args.Get(1).(error)
	if !ok && args.Get(1) != nil {
		return nil, fmt.Errorf("unexpected type for second argument")
	}

	return idToken, err
}

// Ensure MockTokenVerifier implements oidc.IDTokenVerifier
//var _ oidc.IDTokenVerifier = &MockTokenVerifier{}

func TestExtractToken(t *testing.T) {
	tt := []struct {
		name      string
		input     string
		expect    string
		expectErr bool
	}{
		{name: "Valid token", input: "Bearer abcd", expect: "abcd", expectErr: false},
		{name: "Invalid scheme", input: "Bear abcd", expect: "", expectErr: true},
		{name: "No token", input: "Bearer ", expect: "", expectErr: true},
		{name: "Invalid format", input: "abcd", expect: "", expectErr: true},
		{name: "Missing header", input: "", expect: "", expectErr: true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			token, err := ExtractToken(tc.input)
			if tc.expectErr && err == nil {
				t.Fatal("expected an error but did not get one")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("did not expect an error but got one: %v", err)
			}
			if token != tc.expect {
				t.Fatalf("expected %v but got %v", tc.expect, token)
			}
		})
	}
}
