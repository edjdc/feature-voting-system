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
	"github.com/edivilsondalacosta/feature-voting-system/internal/middleware"
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	pgRepo "github.com/edivilsondalacosta/feature-voting-system/internal/repository/postgres"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

func TestSubmitIntegration_ScenarioA(t *testing.T) {
	ctx := context.Background()
	pool := requirePostgres(t, ctx)
	defer pool.Close()

	log := observability.NewLogger()

	userRepo := pgRepo.NewUserRepo(pool)
	requestRepo := pgRepo.NewRequestRepo(pool)
	authSvc := service.NewAuthService("access-secret", "refresh-secret", time.Minute, time.Hour)
	userSvc := service.NewUserService(userRepo, authSvc, log)
	requestSvc := service.NewRequestService(requestRepo, log)
	rankingSvc := service.NewRankingService(requestRepo, log)

	authH := handler.NewAuthHandler(authSvc, userSvc, log)
	requestsH := handler.NewRequestsHandler(requestSvc, rankingSvc, log)

	// Register Alice
	regBody := `{"email":"alice-submit@example.com","password":"password"}`
	regReq := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	authH.Register(regW, regReq)
	require.Equal(t, http.StatusCreated, regW.Code)

	var regResp map[string]string
	require.NoError(t, json.NewDecoder(regW.Body).Decode(&regResp))
	accessToken := regResp["access_token"]

	// Verify access token gives user ID
	claims, err := authSvc.VerifyAccessToken(accessToken)
	require.NoError(t, err)

	t.Run("submit_valid_request", func(t *testing.T) {
		body := `{"title":"Dark mode","description":"Please add a dark theme."}`
		req := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)

		// Inject user ID via context (simulating middleware)
		req = req.WithContext(withUserID(req.Context(), claims.UserID))

		w := httptest.NewRecorder()
		requestsH.Submit(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
		var fr map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&fr))
		assert.Equal(t, float64(0), fr["vote_count"])
		assert.Equal(t, claims.UserID, fr["author_id"])
	})

	t.Run("submit_empty_title_returns_400", func(t *testing.T) {
		body := `{"title":"","description":"Some description."}`
		req := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewBufferString(body))
		req = req.WithContext(withUserID(req.Context(), claims.UserID))
		w := httptest.NewRecorder()
		requestsH.Submit(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("submit_without_auth_returns_401", func(t *testing.T) {
		body := `{"title":"Some feature","description":"Some description."}`
		req := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		requestsH.Submit(w, req) // no user ID in context
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func withUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, middleware.UserIDKey, userID)
}
