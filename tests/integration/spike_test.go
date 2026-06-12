//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/handler"
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	pgRepo "github.com/edivilsondalacosta/feature-voting-system/internal/repository/postgres"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

func TestSpikeIntegration_ScenarioF(t *testing.T) {
	if testing.Short() {
		t.Skip("spike test skipped in short mode")
	}

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

	// Register author and create a request
	author, _ := registerUser(t, authH, authSvc, "author-spike@example.com")
	submitBody, _ := json.Marshal(map[string]string{
		"title":       "Spike test request",
		"description": "This request gets hammered",
	})
	submitReq := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewReader(submitBody))
	submitReq.Header.Set("Content-Type", "application/json")
	submitReq = submitReq.WithContext(withUserID(submitReq.Context(), author.UserID))
	submitW := httptest.NewRecorder()
	requestsH.Submit(submitW, submitReq)
	require.Equal(t, http.StatusCreated, submitW.Code)

	var fr map[string]any
	require.NoError(t, json.NewDecoder(submitW.Body).Decode(&fr))
	requestID := fr["id"].(string)

	const numVoters = 1000

	// Register 1000 distinct voters
	voters := make([]string, numVoters)
	for i := 0; i < numVoters; i++ {
		email := fmt.Sprintf("voter-%d-spike@example.com", i)
		user, _ := registerUser(t, authH, authSvc, email)
		voters[i] = user.UserID
	}

	// Concurrently upvote from all 1000 voters
	var wg sync.WaitGroup
	for _, voterID := range voters {
		wg.Add(1)
		go func(uid string) {
			defer wg.Done()
			req := setChiParam(httptest.NewRequest(http.MethodPut, "/requests/"+requestID+"/vote", nil), "id", requestID)
			req = req.WithContext(withUserID(req.Context(), uid))
			w := httptest.NewRecorder()
			votesH.Upvote(w, req)
		}(voterID)
	}
	wg.Wait()

	// Verify final vote_count == 1000 exactly
	req := setChiParam(httptest.NewRequest(http.MethodGet, "/requests/"+requestID, nil), "id", requestID)
	req = req.WithContext(withUserID(req.Context(), author.UserID))
	w := httptest.NewRecorder()
	requestsH.GetByID(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, float64(numVoters), result["vote_count"],
		"vote_count should equal number of distinct voters exactly")

	// Reconciliation check: vote_count == COUNT(votes)
	var actualCount int32
	err := pool.QueryRow(ctx, "SELECT COUNT(*)::INTEGER FROM votes WHERE request_id = $1", requestID).Scan(&actualCount)
	require.NoError(t, err)
	assert.Equal(t, int32(numVoters), actualCount, "reconciliation check: COUNT(votes) should equal vote_count")
}
