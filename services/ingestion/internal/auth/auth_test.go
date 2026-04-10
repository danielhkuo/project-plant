package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/danielkuo/project-plant/services/ingestion/internal/auth"
)

func testAuthenticator() *auth.StaticKeyAuthenticator {
	return auth.NewStaticKeyAuthenticator(map[string]auth.DeviceIdentity{
		"key-dev-001": {DeviceID: "dev-001"},
		"key-dev-002": {DeviceID: "dev-002"},
		"key-expired": {DeviceID: "dev-003", ExpiresAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
}

func TestValidAPIKey(t *testing.T) {
	authenticator := testAuthenticator()
	identity, err := authenticator.Authenticate("key-dev-001")
	require.NoError(t, err)
	assert.Equal(t, "dev-001", identity.DeviceID)
}

func TestMissingAPIKey(t *testing.T) {
	authenticator := testAuthenticator()
	_, err := authenticator.Authenticate("")
	require.ErrorIs(t, err, auth.ErrMissingAPIKey)
}

func TestInvalidAPIKey(t *testing.T) {
	authenticator := testAuthenticator()
	_, err := authenticator.Authenticate("wrong-key")
	require.ErrorIs(t, err, auth.ErrInvalidAPIKey)
}

func TestExpiredAPIKey(t *testing.T) {
	authenticator := testAuthenticator()
	_, err := authenticator.Authenticate("key-expired")
	require.ErrorIs(t, err, auth.ErrExpiredAPIKey)
}

func TestDeviceIDMismatch(t *testing.T) {
	// This test validates that the auth layer correctly identifies
	// each device — verifying key-dev-001 maps to dev-001, not dev-002
	authenticator := testAuthenticator()
	identity, err := authenticator.Authenticate("key-dev-001")
	require.NoError(t, err)
	assert.Equal(t, "dev-001", identity.DeviceID)
	assert.NotEqual(t, "dev-002", identity.DeviceID)
}

func TestAuthMiddleware_PassesThrough(t *testing.T) {
	reached := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.Middleware(testAuthenticator())(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", nil)
	req.Header.Set("X-API-Key", "key-dev-001")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, reached)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_BlocksUnauthed(t *testing.T) {
	reached := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	})

	handler := auth.Middleware(testAuthenticator())(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, reached)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDeviceIdentityFromContext(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deviceID, ok := auth.DeviceIDFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, "dev-001", deviceID)
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.Middleware(testAuthenticator())(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", nil)
	req.Header.Set("X-API-Key", "key-dev-001")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestStaticAuthenticator_MultipleDevices(t *testing.T) {
	authenticator := testAuthenticator()

	id1, err := authenticator.Authenticate("key-dev-001")
	require.NoError(t, err)
	assert.Equal(t, "dev-001", id1.DeviceID)

	id2, err := authenticator.Authenticate("key-dev-002")
	require.NoError(t, err)
	assert.Equal(t, "dev-002", id2.DeviceID)
}
