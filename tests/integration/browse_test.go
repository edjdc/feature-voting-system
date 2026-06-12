//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func TestBrowseIntegration_ScenarioB(t *testing.T) {
	ctx := context.Background()
	pool := requirePostgres(t, ctx)
	defer pool.Close()

	log := observability.NewLogger()
	authSvc := service.NewAuthService("access-secret", "refresh-secret", time.Minute, time.Hour)

	userRepo := pgRepo.NewUserRepo(pool)
	requestRepo := pgRepo.NewRequestRepo(pool)

	userSvc := service.NewUserService(userRepo, authSvc, log)
	requestSvc := service.NewRequestService(requestRepo, log)
	rankingSvc := service.NewRankingService(requestRepo, log)

	authH := handler.NewAuthHandler(authSvc, userSvc, log)
	requestsH := handler.NewRequestsHandler(requestSvc, rankingSvc, log)

	user, _ := registerUser(t, authH, authSvc, "browse-user@example.com")

	// Seed 25 requests
	for i := 0; i < 25; i++ {
		body, _ := json.Marshal(map[string]string{
			"title":       fmt.Sprintf("Feature %d", i),
			"description": fmt.Sprintf("Description for feature %d", i),
		})
		req := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(withUserID(req.Context(), user.UserID))
		w := httptest.NewRecorder()
		requestsH.Submit(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	}

	t.Run("first_page_has_20_items_and_next_cursor", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/requests?sort=new&limit=20", nil)
		req = req.WithContext(withUserID(req.Context(), user.UserID))
		w := httptest.NewRecorder()
		requestsH.List(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var page map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&page))
		items := page["items"].([]any)
		assert.Len(t, items, 20)
		assert.NotNil(t, page["next_cursor"], "next_cursor should not be nil for a full page")
	})

	t.Run("second_page_no_overlap", func(t *testing.T) {
		// Get first page
		req1 := httptest.NewRequest(http.MethodGet, "/requests?sort=new&limit=20", nil)
		req1 = req1.WithContext(withUserID(req1.Context(), user.UserID))
		w1 := httptest.NewRecorder()
		requestsH.List(w1, req1)
		var page1 map[string]any
		require.NoError(t, json.NewDecoder(w1.Body).Decode(&page1))
		cursor := page1["next_cursor"].(string)

		// Get second page
		req2 := httptest.NewRequest(http.MethodGet, "/requests?sort=new&limit=20&cursor="+cursor, nil)
		req2 = req2.WithContext(withUserID(req2.Context(), user.UserID))
		w2 := httptest.NewRecorder()
		requestsH.List(w2, req2)
		var page2 map[string]any
		require.NoError(t, json.NewDecoder(w2.Body).Decode(&page2))

		items1 := page1["items"].([]any)
		items2 := page2["items"].([]any)

		// Collect IDs from page 1
		ids1 := map[string]bool{}
		for _, item := range items1 {
			m := item.(map[string]any)
			ids1[m["id"].(string)] = true
		}

		// Verify no overlap
		for _, item := range items2 {
			m := item.(map[string]any)
			id := m["id"].(string)
			assert.False(t, ids1[id], "item %s appeared in both pages", id)
		}
		assert.NotEmpty(t, items2)
	})

	t.Run("page_past_end_returns_empty_null_cursor", func(t *testing.T) {
		// Use a fake cursor that would be past the last item
		// Encode a cursor with a very old created_at time
		oldCursor := handler.EncodeCursor("00000000-0000-0000-0000-000000000000",
			time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), 0)
		req := httptest.NewRequest(http.MethodGet, "/requests?sort=new&limit=20&cursor="+oldCursor, nil)
		req = req.WithContext(withUserID(req.Context(), user.UserID))
		w := httptest.NewRecorder()
		requestsH.List(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var page map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&page))
		items := page["items"]
		assert.Nil(t, items)
		assert.Nil(t, page["next_cursor"])
	})
}
