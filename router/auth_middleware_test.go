package router

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

// MockProvider implements router.OIDCProvider interface for testing
type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) Verifier(config *oidc.Config) *oidc.IDTokenVerifier {
	args := m.Called(config)
	return args.Get(0).(*oidc.IDTokenVerifier)
}

// mockOIDCProvider wraps MockProvider to satisfy *oidc.Provider parameter requirements
type mockOIDCProvider struct {
	verifier func(*oidc.Config) *oidc.IDTokenVerifier
}

func (m *mockOIDCProvider) Verifier(config *oidc.Config) *oidc.IDTokenVerifier {
	if m.verifier != nil {
		return m.verifier(config)
	}
	return &oidc.IDTokenVerifier{}
}

type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Log(msg string) {
	m.Called(msg)
}

func (m *MockLogger) LogDebug(msg string) {
	m.Called(msg)
}

func (m *MockLogger) LogActivity(msg string, keyvals ...interface{}) {
	m.Called(msg, keyvals)
}

func (m *MockLogger) LogError(msg string, keyvals ...interface{}) {
	m.Called(msg, keyvals)
}

func (m *MockLogger) LogInfo(msg string, keyvals ...interface{}) {
	m.Called(msg, keyvals)
}

func (m *MockLogger) LogWarn(msg string, keyvals ...interface{}) {
	m.Called(msg, keyvals)
}

// Helper function to create a test JWT token
func createTestJWT(claims jwt.MapClaims, signingMethod jwt.SigningMethod, key interface{}) (string, error) {
	token := jwt.NewWithClaims(signingMethod, claims)
	return token.SignedString(key)
}

// Helper function to create RSA key pair for testing
func createTestRSAKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
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

func TestNewAuthMiddlewareWithConfig_VAPTCompliance(t *testing.T) {
	t.Run("StrictMode_RequiresIssuerURL", func(t *testing.T) {
		mockProvider := &MockProvider{}
		mockCache := &MockTokenCache{}
		mockLogger := &MockLogger{}

		// Test that strict mode requires issuer URL
		_, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:     "test-client",
			Provider:     mockProvider,
			Cache:        mockCache,
			Logger:       mockLogger,
			SecurityMode: StrictMode,
			// IssuerURL missing - should fail
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "IssuerURL is required in StrictMode")
	})

	t.Run("StrictMode_DefaultSecureAlgorithms", func(t *testing.T) {
		mockProvider := &MockProvider{}
		mockCache := &MockTokenCache{}
		mockLogger := &MockLogger{}
		mockVerifier := &oidc.IDTokenVerifier{}

		mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
		mockLogger.On("Log", mock.AnythingOfType("string"))

		auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:     "test-client",
			Provider:     mockProvider,
			Cache:        mockCache,
			Logger:       mockLogger,
			SecurityMode: StrictMode,
			IssuerURL:    "https://keycloak.example.com/realms/test",
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"RS256", "RS384", "RS512"}, auth.AllowedAlgorithms)
		assert.Equal(t, []string{"exp", "iss", "sub"}, auth.RequiredClaims)
		assert.True(t, auth.StoreClaimsInContext)
	})

	t.Run("CompatibilityMode_BackwardCompatible", func(t *testing.T) {
		mockProvider := &MockProvider{}
		mockCache := &MockTokenCache{}
		mockLogger := &MockLogger{}
		mockVerifier := &oidc.IDTokenVerifier{}

		mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
		mockLogger.On("Log", mock.AnythingOfType("string"))

		auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:             "test-client",
			Provider:             mockProvider,
			Cache:                mockCache,
			Logger:               mockLogger,
			SecurityMode:         CompatibilityMode,
			IssuerURL:            "", // Not required in compatibility mode
			StoreClaimsInContext: false,
		})

		require.NoError(t, err)
		assert.Equal(t, CompatibilityMode, auth.SecurityMode)
		assert.False(t, auth.StoreClaimsInContext)
	})

	t.Run("RequiredFields_MissingClientID", func(t *testing.T) {
		mockProvider := &MockProvider{}
		mockCache := &MockTokenCache{}
		mockLogger := &MockLogger{}

		_, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			Provider:     mockProvider,
			Cache:        mockCache,
			Logger:       mockLogger,
			SecurityMode: StrictMode,
			IssuerURL:    "https://keycloak.example.com/realms/test",
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ClientID is required")
	})

	t.Run("RequiredFields_MissingProvider", func(t *testing.T) {
		mockCache := &MockTokenCache{}
		mockLogger := &MockLogger{}

		_, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:     "test-client",
			Cache:        mockCache,
			Logger:       mockLogger,
			SecurityMode: StrictMode,
			IssuerURL:    "https://keycloak.example.com/realms/test",
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Provider is required")
	})

	t.Run("RequiredFields_MissingCache", func(t *testing.T) {
		mockProvider := &MockProvider{}
		mockLogger := &MockLogger{}

		_, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:     "test-client",
			Provider:     mockProvider,
			Logger:       mockLogger,
			SecurityMode: StrictMode,
			IssuerURL:    "https://keycloak.example.com/realms/test",
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Cache is required")
	})

	t.Run("CustomAlgorithms_Applied", func(t *testing.T) {
		mockProvider := &MockProvider{}
		mockCache := &MockTokenCache{}
		mockLogger := &MockLogger{}
		mockVerifier := &oidc.IDTokenVerifier{}

		mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
		mockLogger.On("Log", mock.AnythingOfType("string"))

		customAlgorithms := []string{"RS384", "RS512"}
		auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:          "test-client",
			Provider:          mockProvider,
			Cache:             mockCache,
			Logger:            mockLogger,
			SecurityMode:      StrictMode,
			IssuerURL:         "https://keycloak.example.com/realms/test",
			AllowedAlgorithms: customAlgorithms,
		})

		require.NoError(t, err)
		assert.Equal(t, customAlgorithms, auth.AllowedAlgorithms)
	})

	t.Run("CustomRequiredClaims_Applied", func(t *testing.T) {
		mockProvider := &MockProvider{}
		mockCache := &MockTokenCache{}
		mockLogger := &MockLogger{}
		mockVerifier := &oidc.IDTokenVerifier{}

		mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
		mockLogger.On("Log", mock.AnythingOfType("string"))

		customClaims := []string{"exp", "iss", "sub", "email", "role"}
		auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:       "test-client",
			Provider:       mockProvider,
			Cache:          mockCache,
			Logger:         mockLogger,
			SecurityMode:   StrictMode,
			IssuerURL:      "https://keycloak.example.com/realms/test",
			RequiredClaims: customClaims,
		})

		require.NoError(t, err)
		assert.Equal(t, customClaims, auth.RequiredClaims)
	})

	t.Run("StrictMode_EnforcesAllSecurityChecks", func(t *testing.T) {
		mockProvider := &MockProvider{}
		mockCache := &MockTokenCache{}
		mockLogger := &MockLogger{}
		mockVerifier := &oidc.IDTokenVerifier{}

		var capturedConfig *oidc.Config
		mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Run(func(args mock.Arguments) {
			capturedConfig = args.Get(0).(*oidc.Config)
		}).Return(mockVerifier)
		mockLogger.On("Log", mock.AnythingOfType("string"))

		_, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:     "test-client",
			Provider:     mockProvider,
			Cache:        mockCache,
			Logger:       mockLogger,
			SecurityMode: StrictMode,
			IssuerURL:    "https://keycloak.example.com/realms/test",
		})

		require.NoError(t, err)
		assert.False(t, capturedConfig.SkipClientIDCheck, "Strict mode should enforce client ID check")
		assert.False(t, capturedConfig.SkipExpiryCheck, "Strict mode should enforce expiry check")
		assert.False(t, capturedConfig.SkipIssuerCheck, "Strict mode should enforce issuer check")
		assert.Equal(t, []string{"RS256", "RS384", "RS512"}, capturedConfig.SupportedSigningAlgs)
	})
}

func TestValidateAlgorithm(t *testing.T) {
	privateKey, err := createTestRSAKey()
	require.NoError(t, err)

	mockProvider := &MockProvider{}
	mockCache := &MockTokenCache{}
	mockLogger := &MockLogger{}
	mockVerifier := &oidc.IDTokenVerifier{}

	mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
	mockLogger.On("Log", mock.AnythingOfType("string"))

	auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
		ClientID:          "test-client",
		Provider:          mockProvider,
		Cache:             mockCache,
		Logger:            mockLogger,
		SecurityMode:      StrictMode,
		IssuerURL:         "https://keycloak.example.com/realms/test",
		AllowedAlgorithms: []string{"RS256"},
	})
	require.NoError(t, err)

	t.Run("AllowedAlgorithm_Success", func(t *testing.T) {
		claims := jwt.MapClaims{"sub": "test"}
		token, err := createTestJWT(claims, jwt.SigningMethodRS256, privateKey)
		require.NoError(t, err)

		err = auth.validateAlgorithm(token)
		assert.NoError(t, err)
	})

	t.Run("DisallowedAlgorithm_Failure", func(t *testing.T) {
		claims := jwt.MapClaims{"sub": "test"}
		token, err := createTestJWT(claims, jwt.SigningMethodHS256, []byte("secret"))
		require.NoError(t, err)

		err = auth.validateAlgorithm(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "algorithm HS256 not allowed")
	})
}

func TestValidateClaims_VAPTCompliance(t *testing.T) {
	mockProvider := &MockProvider{}
	mockCache := &MockTokenCache{}
	mockLogger := &MockLogger{}
	mockVerifier := &oidc.IDTokenVerifier{}

	mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
	mockLogger.On("Log", mock.AnythingOfType("string"))

	auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
		ClientID:     "test-client",
		Provider:     mockProvider,
		Cache:        mockCache,
		Logger:       mockLogger,
		SecurityMode: StrictMode,
		IssuerURL:    "https://keycloak.example.com/realms/test",
	})
	require.NoError(t, err)

	now := time.Now()

	t.Run("ValidClaims_Success", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp": float64(now.Add(time.Hour).Unix()),
			"iss": "https://keycloak.example.com/realms/test",
			"sub": "test-user",
			"iat": float64(now.Add(-time.Minute).Unix()),
		}

		err := auth.validateClaims(claims)
		assert.NoError(t, err)
	})

	t.Run("ExpiredToken_Failure", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp": float64(now.Add(-time.Hour).Unix()), // Expired
			"iss": "https://keycloak.example.com/realms/test",
			"sub": "test-user",
		}

		err := auth.validateClaims(claims)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token expired")
	})

	t.Run("InvalidIssuer_Failure", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp": float64(now.Add(time.Hour).Unix()),
			"iss": "https://malicious.example.com/realms/fake", // Wrong issuer
			"sub": "test-user",
		}

		err := auth.validateClaims(claims)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid issuer")
	})

	t.Run("MissingRequiredClaim_Failure", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp": float64(now.Add(time.Hour).Unix()),
			"iss": "https://keycloak.example.com/realms/test",
			// Missing "sub" claim
		}

		err := auth.validateClaims(claims)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token missing required claim: sub")
	})

	t.Run("FutureIssuedAt_Failure", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp": float64(now.Add(time.Hour).Unix()),
			"iss": "https://keycloak.example.com/realms/test",
			"sub": "test-user",
			"iat": float64(now.Add(time.Hour).Unix()), // Future issued at
		}

		err := auth.validateClaims(claims)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token issued in the future")
	})

	t.Run("NotBeforeInFuture_Failure", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp": float64(now.Add(time.Hour).Unix()),
			"iss": "https://keycloak.example.com/realms/test",
			"sub": "test-user",
			"nbf": float64(now.Add(time.Hour).Unix()), // Not valid yet
		}

		err := auth.validateClaims(claims)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token not valid before")
	})
}

func TestMiddlewareFunc_StrictMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockProvider := &MockProvider{}
	mockCache := &MockTokenCache{}
	mockLogger := &MockLogger{}
	mockVerifier := &oidc.IDTokenVerifier{}

	mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
	mockLogger.On("Log", mock.AnythingOfType("string"))
	mockLogger.On("LogDebug", mock.AnythingOfType("string"))

	auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
		ClientID:     "test-client",
		Provider:     mockProvider,
		Cache:        mockCache,
		Logger:       mockLogger,
		SecurityMode: StrictMode,
		IssuerURL:    "https://keycloak.example.com/realms/test",
	})
	require.NoError(t, err)

	// Set up error scenarios for testing
	RegisterAuthMsgID(TokenMissing, 1001)
	RegisterAuthErrCode(TokenMissing, "TOKEN_MISSING")
	RegisterAuthMsgID(TokenVerificationFailed, 1002)
	RegisterAuthErrCode(TokenVerificationFailed, "TOKEN_INVALID")

	t.Run("MissingToken_ReturnsUnauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		// No Authorization header

		auth.MiddlewareFunc()(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.True(t, c.IsAborted())
	})

	t.Run("InvalidToken_ReturnsUnauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		c.Request.Header.Set("Authorization", "Bearer invalid-token")

		auth.MiddlewareFunc()(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.True(t, c.IsAborted())
	})
}

func TestCustomClaimsValidation(t *testing.T) {
	mockProvider := &MockProvider{}
	mockCache := &MockTokenCache{}
	mockLogger := &MockLogger{}
	mockVerifier := &oidc.IDTokenVerifier{}

	mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
	mockLogger.On("Log", mock.AnythingOfType("string"))

	// Custom validator that requires "role" claim
	customValidator := func(claims jwt.MapClaims) error {
		if role, ok := claims["role"].(string); !ok || role != "admin" {
			return fmt.Errorf("insufficient privileges: admin role required")
		}
		return nil
	}

	auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
		ClientID:           "test-client",
		Provider:           mockProvider,
		Cache:              mockCache,
		Logger:             mockLogger,
		SecurityMode:       StrictMode,
		IssuerURL:          "https://keycloak.example.com/realms/test",
		ValidateClaimsFunc: customValidator,
	})
	require.NoError(t, err)

	now := time.Now()

	t.Run("CustomValidation_Success", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp":  float64(now.Add(time.Hour).Unix()),
			"iss":  "https://keycloak.example.com/realms/test",
			"sub":  "test-user",
			"role": "admin", // Required by custom validator
		}

		err := auth.validateClaims(claims)
		assert.NoError(t, err)

		// Test custom validation directly
		err = auth.ValidateClaimsFunc(claims)
		assert.NoError(t, err)
	})

	t.Run("CustomValidation_Failure", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp":  float64(now.Add(time.Hour).Unix()),
			"iss":  "https://keycloak.example.com/realms/test",
			"sub":  "test-user",
			"role": "user", // Insufficient role
		}

		err := auth.ValidateClaimsFunc(claims)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient privileges")
	})
}

func TestMiddlewareFunc_CompatibilityMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockProvider := &MockProvider{}
	mockCache := &MockTokenCache{}
	mockLogger := &MockLogger{}
	mockVerifier := &oidc.IDTokenVerifier{}

	mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
	mockLogger.On("Log", mock.AnythingOfType("string")).Maybe()

	// Create auth middleware in compatibility mode
	auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
		ClientID:             "test-client",
		Provider:             mockProvider,
		Cache:                mockCache,
		Logger:               mockLogger,
		SecurityMode:         CompatibilityMode,
		StoreClaimsInContext: false,
	})
	require.NoError(t, err)
	assert.Equal(t, CompatibilityMode, auth.SecurityMode)

	// Set up error scenarios
	RegisterAuthMsgID(TokenCacheFailed, 1003)
	RegisterAuthErrCode(TokenCacheFailed, "CACHE_ERROR")
	RegisterAuthMsgID(TokenVerificationFailed, 1002)
	RegisterAuthErrCode(TokenVerificationFailed, "TOKEN_INVALID")

	t.Run("CompatibilityMode_UsesCache", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		c.Request.Header.Set("Authorization", "Bearer test-token")

		// Mock cache to return token exists
		mockCache.On("Get", "test-token").Return(true, nil).Once()

		auth.MiddlewareFunc()(c)

		// In compatibility mode with cached token, should succeed without verification
		assert.False(t, c.IsAborted())
		mockCache.AssertExpectations(t)
	})

	t.Run("CompatibilityMode_CacheMiss_VerifiesToken", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		c.Request.Header.Set("Authorization", "Bearer new-token")

		// Mock cache miss, then verification fails
		mockCache.On("Get", "new-token").Return(false, nil).Once()

		auth.MiddlewareFunc()(c)

		// Should attempt verification when not in cache
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		mockCache.AssertExpectations(t)
	})
}

func TestMiddlewareFunc_ContextClaimsStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("StrictMode_StoresClaimsInContext", func(t *testing.T) {
		mockProvider := &MockProvider{}
		mockCache := &MockTokenCache{}
		mockLogger := &MockLogger{}
		mockVerifier := &oidc.IDTokenVerifier{}

		mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
		mockLogger.On("Log", mock.AnythingOfType("string"))
		mockLogger.On("LogDebug", mock.AnythingOfType("string")).Maybe()

		auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:             "test-client",
			Provider:             mockProvider,
			Cache:                mockCache,
			Logger:               mockLogger,
			SecurityMode:         StrictMode,
			IssuerURL:            "https://keycloak.example.com/realms/test",
			StoreClaimsInContext: true,
		})
		require.NoError(t, err)

		// Test that claims would be stored (we can't fully test this without a real OIDC token)
		// But we can verify the configuration
		assert.True(t, auth.StoreClaimsInContext)
	})

	t.Run("CompatibilityMode_DoesNotStoreClaimsByDefault", func(t *testing.T) {
		mockProvider := &MockProvider{}
		mockCache := &MockTokenCache{}
		mockLogger := &MockLogger{}
		mockVerifier := &oidc.IDTokenVerifier{}

		mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
		mockLogger.On("Log", mock.AnythingOfType("string")).Maybe()

		auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:             "test-client",
			Provider:             mockProvider,
			Cache:                mockCache,
			Logger:               mockLogger,
			SecurityMode:         CompatibilityMode,
			StoreClaimsInContext: false,
		})
		require.NoError(t, err)

		assert.False(t, auth.StoreClaimsInContext)
	})
}

func TestValidateClaims_EdgeCases(t *testing.T) {
	mockProvider := &MockProvider{}
	mockCache := &MockTokenCache{}
	mockLogger := &MockLogger{}
	mockVerifier := &oidc.IDTokenVerifier{}

	mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
	mockLogger.On("Log", mock.AnythingOfType("string"))

	auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
		ClientID:     "test-client",
		Provider:     mockProvider,
		Cache:        mockCache,
		Logger:       mockLogger,
		SecurityMode: StrictMode,
		IssuerURL:    "https://keycloak.example.com/realms/test",
	})
	require.NoError(t, err)

	now := time.Now()

	t.Run("MissingExpClaim_Failure", func(t *testing.T) {
		claims := jwt.MapClaims{
			"iss": "https://keycloak.example.com/realms/test",
			"sub": "test-user",
			// exp missing
		}

		err := auth.validateClaims(claims)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token missing required claim: exp")
	})

	t.Run("MissingIssuerClaim_Failure", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp": float64(now.Add(time.Hour).Unix()),
			"sub": "test-user",
			// iss missing
		}

		err := auth.validateClaims(claims)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token missing required claim: iss")
	})

	t.Run("ValidNbf_Success", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp": float64(now.Add(time.Hour).Unix()),
			"iss": "https://keycloak.example.com/realms/test",
			"sub": "test-user",
			"nbf": float64(now.Add(-time.Minute).Unix()), // Valid from 1 minute ago
		}

		err := auth.validateClaims(claims)
		assert.NoError(t, err)
	})

	t.Run("ValidIat_Success", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp": float64(now.Add(time.Hour).Unix()),
			"iss": "https://keycloak.example.com/realms/test",
			"sub": "test-user",
			"iat": float64(now.Add(-time.Minute).Unix()), // Issued 1 minute ago
		}

		err := auth.validateClaims(claims)
		assert.NoError(t, err)
	})

	t.Run("ClockSkewGracePeriod_IatWithinSkew", func(t *testing.T) {
		claims := jwt.MapClaims{
			"exp": float64(now.Add(time.Hour).Unix()),
			"iss": "https://keycloak.example.com/realms/test",
			"sub": "test-user",
			"iat": float64(now.Add(3 * time.Second).Unix()), // 3 seconds in future (within 5s grace)
		}

		err := auth.validateClaims(claims)
		assert.NoError(t, err)
	})
}

func TestSecurityModeDefaults(t *testing.T) {
	mockProvider := &MockProvider{}
	mockCache := &MockTokenCache{}
	mockLogger := &MockLogger{}
	mockVerifier := &oidc.IDTokenVerifier{}

	mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
	mockLogger.On("Log", mock.AnythingOfType("string"))

	t.Run("DefaultsToStrictMode", func(t *testing.T) {
		auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
			ClientID:  "test-client",
			Provider:  mockProvider,
			Cache:     mockCache,
			Logger:    mockLogger,
			IssuerURL: "https://keycloak.example.com/realms/test",
			// SecurityMode not specified - should default to StrictMode
		})

		require.NoError(t, err)
		assert.Equal(t, StrictMode, auth.SecurityMode)
	})
}

func TestAlgorithmValidation_MultipleAlgorithms(t *testing.T) {
	privateKey, err := createTestRSAKey()
	require.NoError(t, err)

	mockProvider := &MockProvider{}
	mockCache := &MockTokenCache{}
	mockLogger := &MockLogger{}
	mockVerifier := &oidc.IDTokenVerifier{}

	mockProvider.On("Verifier", mock.AnythingOfType("*oidc.Config")).Return(mockVerifier)
	mockLogger.On("Log", mock.AnythingOfType("string"))

	auth, err := NewAuthMiddlewareWithConfig(AuthMiddlewareConfig{
		ClientID:          "test-client",
		Provider:          mockProvider,
		Cache:             mockCache,
		Logger:            mockLogger,
		SecurityMode:      StrictMode,
		IssuerURL:         "https://keycloak.example.com/realms/test",
		AllowedAlgorithms: []string{"RS256", "RS384", "RS512"},
	})
	require.NoError(t, err)

	t.Run("RS384_Allowed", func(t *testing.T) {
		claims := jwt.MapClaims{"sub": "test"}
		token, err := createTestJWT(claims, jwt.SigningMethodRS384, privateKey)
		require.NoError(t, err)

		err = auth.validateAlgorithm(token)
		assert.NoError(t, err)
	})

	t.Run("RS512_Allowed", func(t *testing.T) {
		claims := jwt.MapClaims{"sub": "test"}
		token, err := createTestJWT(claims, jwt.SigningMethodRS512, privateKey)
		require.NoError(t, err)

		err = auth.validateAlgorithm(token)
		assert.NoError(t, err)
	})

	t.Run("PS256_NotAllowed", func(t *testing.T) {
		claims := jwt.MapClaims{"sub": "test"}
		token, err := createTestJWT(claims, jwt.SigningMethodPS256, privateKey)
		require.NoError(t, err)

		err = auth.validateAlgorithm(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "algorithm PS256 not allowed")
	})
}
