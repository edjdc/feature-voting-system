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
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	pgRepo "github.com/edivilsondalacosta/feature-voting-system/internal/repository/postgres"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

func TestConditionalGetIntegration_ScenarioE(t *testing.T) {
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

	// Register users
	alice, _ := registerUser(t, authH, authSvc, "alice-conditional@example.com")
	bob, _ := registerUser(t, authH, authSvc, "bob-conditional@example.com")

	// Alice submits a request
	submitBody, _ := json.Marshal(map[string]string{
		"title":       "Conditional test request",
		"description": "Testing ETag behavior",
	})
	submitReq := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewReader(submitBody))
	submitReq.Header.Set("Content-Type", "application/json")
	submitReq = submitReq.WithContext(withUserID(submitReq.Context(), alice.UserID))
	submitW := httptest.NewRecorder()
	requestsH.Submit(submitW, submitReq)
	require.Equal(t, http.StatusCreated, submitW.Code)

	var fr map[string]any
	require.NoError(t, json.NewDecoder(submitW.Body).Decode(&fr))
	requestID := fr["id"].(string)

	t.Run("list_returns_etag", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/requests?sort=top", nil)
		req = req.WithContext(withUserID(req.Context(), alice.UserID))
		w := httptest.NewRecorder()
		requestsH.List(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get("ETag"))
		assert.Equal(t, "private, max-age=0, must-revalidate", w.Header().Get("Cache-Control"))
	})

	t.Run("if_none_match_returns_304_when_unchanged", func(t *testing.T) {
		req1 := httptest.NewRequest(http.MethodGet, "/requests?sort=top", nil)
		req1 = req1.WithContext(withUserID(req1.Context(), alice.UserID))
		w1 := httptest.NewRecorder()
		requestsH.List(w1, req1)
		etag := w1.Header().Get("ETag")
		require.NotEmpty(t, etag)

		req2 := httptest.NewRequest(http.MethodGet, "/requests?sort=top", nil)
		req2 = req2.WithContext(withUserID(req2.Context(), alice.UserID))
		req2.Header.Set("If-None-Match", etag)
		w2 := httptest.NewRecorder()
		requestsH.List(w2, req2)
		assert.Equal(t, http.StatusNotModified, w2.Code)
	})

	t.Run("after_vote_etag_changes", func(t *testing.T) {
		// Get ETag before vote
		req1 := httptest.NewRequest(http.MethodGet, "/requests?sort=top", nil)
		req1 = req1.WithContext(withUserID(req1.Context(), alice.UserID))
		w1 := httptest.NewRecorder()
		requestsH.List(w1, req1)
		oldEtag := w1.Header().Get("ETag")

		// Bob votes
		voteReq := setChiParam(httptest.NewRequest(http.MethodPut, "/requests/"+requestID+"/vote", nil), "id", requestID)
		voteReq = voteReq.WithContext(withUserID(voteReq.Context(), bob.UserID))
		voteW := httptest.NewRecorder()
		votesH.Upvote(voteW, voteReq)
		require.Equal(t, http.StatusOK, voteW.Code)

		// Get ETag after vote — should be different
		req2 := httptest.NewRequest(http.MethodGet, "/requests?sort=top", nil)
		req2 = req2.WithContext(withUserID(req2.Context(), alice.UserID))
		req2.Header.Set("If-None-Match", oldEtag)
		w2 := httptest.NewRecorder()
		requestsH.List(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code, "after a vote, response should be 200 with new ETag")
		assert.NotEqual(t, oldEtag, w2.Header().Get("ETag"), "ETag should change after vote")
	})
}
