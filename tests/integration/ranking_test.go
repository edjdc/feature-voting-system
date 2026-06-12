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

	"fmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/handler"
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	pgRepo "github.com/edivilsondalacosta/feature-voting-system/internal/repository/postgres"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

func TestRankingIntegration_ScenarioD(t *testing.T) {
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
	authorA, _ := registerUser(t, authH, authSvc, "author-a-rank@example.com")
	authorB, _ := registerUser(t, authH, authSvc, "author-b-rank@example.com")

	// Create request X (will have votes but be old)
	submitX, _ := json.Marshal(map[string]string{"title": "Request X Old", "description": "Old request"})
	reqX := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewReader(submitX))
	reqX.Header.Set("Content-Type", "application/json")
	reqX = reqX.WithContext(withUserID(reqX.Context(), authorA.UserID))
	wX := httptest.NewRecorder()
	requestsH.Submit(wX, reqX)
	require.Equal(t, http.StatusCreated, wX.Code)
	var frX map[string]any
	require.NoError(t, json.NewDecoder(wX.Body).Decode(&frX))
	requestXID := frX["id"].(string)

	// Create request Y (recent, same vote count)
	submitY, _ := json.Marshal(map[string]string{"title": "Request Y Recent", "description": "Recent request"})
	reqY := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewReader(submitY))
	reqY.Header.Set("Content-Type", "application/json")
	reqY = reqY.WithContext(withUserID(reqY.Context(), authorB.UserID))
	wY := httptest.NewRecorder()
	requestsH.Submit(wY, reqY)
	require.Equal(t, http.StatusCreated, wY.Code)
	var frY map[string]any
	require.NoError(t, json.NewDecoder(wY.Body).Decode(&frY))
	requestYID := frY["id"].(string)

	// Vote for both with same number of voters
	voters := make([]*testUser, 3)
	for i := range voters {
		u, _ := registerUser(t, authH, authSvc, fmt.Sprintf("voter-rank-%d@example.com", i))
		voters[i] = u
	}

	for _, voter := range voters {
		for _, reqID := range []string{requestXID, requestYID} {
			req := setChiParam(httptest.NewRequest(http.MethodPut, "/requests/"+reqID+"/vote", nil), "id", reqID)
			req = req.WithContext(withUserID(req.Context(), voter.UserID))
			w := httptest.NewRecorder()
			votesH.Upvote(w, req)
			require.Equal(t, http.StatusOK, w.Code)
		}
	}

	// Make X appear older by updating its created_at directly
	_, err := pool.Exec(ctx,
		"UPDATE feature_requests SET created_at = NOW() - INTERVAL '48 hours' WHERE id = $1",
		requestXID)
	require.NoError(t, err)

	t.Run("top_sort_stable_tiebreaker", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/requests?sort=top&limit=10", nil)
		req = req.WithContext(withUserID(req.Context(), authorA.UserID))
		w := httptest.NewRecorder()
		requestsH.List(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var page map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&page))
		items := page["items"].([]any)
		require.GreaterOrEqual(t, len(items), 2)
		// Both should have equal vote counts, ordered by created_at DESC
		// Y (recent) should come first in top sort with same votes
		first := items[0].(map[string]any)
		assert.Equal(t, requestYID, first["id"].(string), "newer request should appear first when vote counts are equal")
	})
}
