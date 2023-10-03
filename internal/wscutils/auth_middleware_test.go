package wscutils

import (
	"context"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"testing"
)

var (
	rec *httptest.ResponseRecorder
	c   *gin.Context
)

func setup() {
	// Setting up recorder and context for gin
	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Add("Authorization", "Bearer testToken")
}

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
	return args.Get(0).(*oidc.IDToken), args.Error(1)
}

func TestOIDCAuthMiddleware_WhenTokenNotInCache_VerifyCalled(t *testing.T) {
	mockedVerifier := new(MockTokenVerifier)
	mockedCache := new(MockTokenCache)
	rawIDToken := "testToken"
	authMiddleware := NewAuthMiddleware("clientID", "clientSecret", "http://localhost:8080", mockedVerifier, mockedCache)

	// Setting up recorder and context for gin
	setup()

	mockedCache.On("Get", rawIDToken).Return(false, nil)
	mockedVerifier.On("Verify", context.Background(), rawIDToken).Return(new(oidc.IDToken), nil)
	mockedCache.On("Set", rawIDToken).Return(nil)

	authMiddleware.MiddlewareFunc()(c)

	mockedVerifier.AssertCalled(t, "Verify", context.Background(), rawIDToken)
}

func TestOIDCAuthMiddleware_WhenTokenInCache_VerifyNotCalled(t *testing.T) {
	mockedVerifier := new(MockTokenVerifier)
	mockedCache := new(MockTokenCache)
	rawIDToken := "testToken"
	authMiddleware := NewAuthMiddleware("clientID", "clientSecret", "http://localhost:8080", mockedVerifier, mockedCache)

	// Setting up recorder and context for gin
	setup()

	mockedCache.On("Get", rawIDToken).Return(true, nil)

	authMiddleware.MiddlewareFunc()(c)

	mockedVerifier.AssertNotCalled(t, "Verify", context.Background(), rawIDToken)
}

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
