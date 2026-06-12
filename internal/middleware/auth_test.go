package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/middleware"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	token, err := authSvc.IssueAccessToken("user-123")
	require.NoError(t, err)

	var capturedUserID string
	handler := middleware.Auth(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.GetUserID(r)
		assert.True(t, ok)
		capturedUserID = userID
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user-123", capturedUserID)
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	handler := middleware.Auth(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	handler := middleware.Auth(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_WrongFormat(t *testing.T) {
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	handler := middleware.Auth(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token sometoken")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
