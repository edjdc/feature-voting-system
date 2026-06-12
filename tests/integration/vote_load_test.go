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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/handler"
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	pgRepo "github.com/edivilsondalacosta/feature-voting-system/internal/repository/postgres"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

// TestVoteLoad validates SC-008: ≥95% first-try success rate under 50 concurrent voters.
func TestVoteLoad_SC008(t *testing.T) {
	if testing.Short() {
		t.Skip("load test skipped in short mode")
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
	author, _ := registerUser(t, authH, authSvc, "author-load@example.com")
	submitBody, _ := json.Marshal(map[string]string{
		"title":       "Load test request",
		"description": "50 concurrent voters",
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

	const numVoters = 50

	// Register voters
	voters := make([]string, numVoters)
	for i := range voters {
		u, _ := registerUser(t, authH, authSvc, fmt.Sprintf("voter-load-%d@example.com", i))
		voters[i] = u.UserID
	}

	var successes, failures int64
	var wg sync.WaitGroup

	for _, voterID := range voters {
		wg.Add(1)
		go func(uid string) {
			defer wg.Done()
			req := setChiParam(httptest.NewRequest(http.MethodPut, "/requests/"+requestID+"/vote", nil), "id", requestID)
			req = req.WithContext(withUserID(req.Context(), uid))
			w := httptest.NewRecorder()
			votesH.Upvote(w, req)
			if w.Code == http.StatusOK {
				atomic.AddInt64(&successes, 1)
			} else {
				atomic.AddInt64(&failures, 1)
			}
		}(voterID)
	}
	wg.Wait()

	total := successes + failures
	successRate := float64(successes) / float64(total)
	t.Logf("Load test: %d/%d successes (%.1f%%)", successes, total, successRate*100)

	assert.GreaterOrEqual(t, successRate, 0.95, "success rate must be ≥95%%")
}
