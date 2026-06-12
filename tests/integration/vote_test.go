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

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/handler"
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	pgRepo "github.com/edivilsondalacosta/feature-voting-system/internal/repository/postgres"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

func TestVoteIntegration_ScenarioC(t *testing.T) {
	ctx := context.Background()
	pool := requirePostgres(t, ctx)
	defer pool.Close()

	log := observability.NewLogger()
	authSvc := service.NewAuthService("access-secret", "refresh-secret", time.Minute, time.Hour)

	userRepo := pgRepo.NewUserRepo(pool)
	requestRepo := pgRepo.NewRequestRepo(pool)
	voteRepo := pgRepo.NewVoteRepo(pool)

	userSvc := service.NewUserService(userRepo, authSvc, log)
	requestSvc := service.NewRequestService(requestRepo, log)
	rankingSvc := service.NewRankingService(requestRepo, log)
	voteSvc := service.NewVoteService(voteRepo, log)

	authH := handler.NewAuthHandler(authSvc, userSvc, log)
	requestsH := handler.NewRequestsHandler(requestSvc, rankingSvc, log)
	votesH := handler.NewVotesHandler(voteSvc, log)

	// Register Alice and Bob
	alice, aliceToken := registerUser(t, authH, authSvc, "alice-vote@example.com")
	bob, bobToken := registerUser(t, authH, authSvc, "bob-vote@example.com")
	_ = alice

	// Alice submits a request
	submitReq := httptest.NewRequest(http.MethodPost, "/requests",
		bytes.NewBufferString(`{"title":"Dark mode","description":"Please add dark theme."}`))
	submitReq.Header.Set("Content-Type", "application/json")
	submitReq = submitReq.WithContext(withUserID(submitReq.Context(), alice.UserID))
	submitW := httptest.NewRecorder()
	requestsH.Submit(submitW, submitReq)
	require.Equal(t, http.StatusCreated, submitW.Code)

	var fr map[string]any
	require.NoError(t, json.NewDecoder(submitW.Body).Decode(&fr))
	requestID := fr["id"].(string)

	t.Run("bob_upvotes_alice_request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/requests/"+requestID+"/vote", nil)
		req = req.WithContext(withUserID(req.Context(), bob.UserID))
		w := httptest.NewRecorder()
		w.Header() // init
		// Set URL param via chi context
		req = setChiParam(req, "id", requestID)
		votesH.Upvote(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var result map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
		assert.Equal(t, float64(1), result["vote_count"])
		assert.Equal(t, true, result["viewer_has_voted"])
	})

	t.Run("bob_upvotes_again_idempotent", func(t *testing.T) {
		req := setChiParam(httptest.NewRequest(http.MethodPut, "/requests/"+requestID+"/vote", nil), "id", requestID)
		req = req.WithContext(withUserID(req.Context(), bob.UserID))
		w := httptest.NewRecorder()
		votesH.Upvote(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var result map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
		assert.Equal(t, float64(1), result["vote_count"], "should still be 1 after duplicate vote")
	})

	t.Run("alice_self_vote_rejected", func(t *testing.T) {
		req := setChiParam(httptest.NewRequest(http.MethodPut, "/requests/"+requestID+"/vote", nil), "id", requestID)
		req = req.WithContext(withUserID(req.Context(), alice.UserID))
		w := httptest.NewRecorder()
		votesH.Upvote(w, req)
		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("bob_removes_vote", func(t *testing.T) {
		req := setChiParam(httptest.NewRequest(http.MethodDelete, "/requests/"+requestID+"/vote", nil), "id", requestID)
		req = req.WithContext(withUserID(req.Context(), bob.UserID))
		w := httptest.NewRecorder()
		votesH.RemoveVote(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var result map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
		assert.Equal(t, float64(0), result["vote_count"])
		assert.Equal(t, false, result["viewer_has_voted"])
	})

	t.Run("vote_on_nonexistent_returns_404", func(t *testing.T) {
		req := setChiParam(httptest.NewRequest(http.MethodPut, "/requests/nonexistent/vote", nil), "id", "00000000-0000-0000-0000-000000000000")
		req = req.WithContext(withUserID(req.Context(), bob.UserID))
		w := httptest.NewRecorder()
		votesH.Upvote(w, req)
		require.Equal(t, http.StatusNotFound, w.Code)
	})

	_ = aliceToken
	_ = bobToken
}

type testUser struct {
	UserID string
}

func registerUser(t *testing.T, authH *handler.AuthHandler, authSvc *service.AuthService, email string) (*testUser, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": "password"})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	authH.Register(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	claims, err := authSvc.VerifyAccessToken(resp["access_token"])
	require.NoError(t, err)
	return &testUser{UserID: claims.UserID}, resp["access_token"]
}

func setChiParam(r *http.Request, key, value string) *http.Request {
	// For testing without a full chi router, inject URL param via chi's context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
