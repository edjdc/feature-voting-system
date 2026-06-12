//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/handler"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
	pgRepo "github.com/edivilsondalacosta/feature-voting-system/internal/repository/postgres"
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
)

func TestAuthIntegration(t *testing.T) {
	ctx := context.Background()
	pool := requirePostgres(t, ctx)
	defer pool.Close()

	log := observability.NewLogger()

	userRepo := pgRepo.NewUserRepo(pool)
	authSvc := service.NewAuthService("test-access-secret", "test-refresh-secret", time.Minute, time.Hour)
	userSvc := service.NewUserService(userRepo, authSvc, log)
	authH := handler.NewAuthHandler(authSvc, userSvc, log)

	t.Run("register_and_login", func(t *testing.T) {
		// Register
		body := `{"email":"alice@example.com","password":"correct horse battery"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		authH.Register(w, req)

		resp := w.Result()
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var regResp map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&regResp))
		assert.NotEmpty(t, regResp["access_token"])
		assert.NotEmpty(t, regResp["refresh_token"])

		// Login
		req2 := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		authH.Login(w2, req2)

		resp2 := w2.Result()
		require.Equal(t, http.StatusOK, resp2.StatusCode)

		var loginResp map[string]string
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&loginResp))
		assert.NotEmpty(t, loginResp["access_token"])

		// Refresh
		refreshBody, _ := json.Marshal(map[string]string{"refresh_token": regResp["refresh_token"]})
		req3 := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(refreshBody))
		req3.Header.Set("Content-Type", "application/json")
		w3 := httptest.NewRecorder()
		authH.Refresh(w3, req3)

		resp3 := w3.Result()
		require.Equal(t, http.StatusOK, resp3.StatusCode)

		var refreshResp map[string]string
		require.NoError(t, json.NewDecoder(resp3.Body).Decode(&refreshResp))
		assert.NotEmpty(t, refreshResp["access_token"])
	})

	t.Run("duplicate_email_rejected", func(t *testing.T) {
		body := `{"email":"bob@example.com","password":"password123"}`
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			authH.Register(w, req)
			if i == 0 {
				require.Equal(t, http.StatusCreated, w.Result().StatusCode)
			} else {
				require.Equal(t, http.StatusConflict, w.Result().StatusCode)
			}
		}
	})

	t.Run("wrong_password_rejected", func(t *testing.T) {
		body := `{"email":"carol@example.com","password":"correct"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		authH.Register(w, req)
		require.Equal(t, http.StatusCreated, w.Result().StatusCode)

		wrongBody := `{"email":"carol@example.com","password":"wrong"}`
		req2 := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(wrongBody))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		authH.Login(w2, req2)
		require.Equal(t, http.StatusUnauthorized, w2.Result().StatusCode)
	})
}
