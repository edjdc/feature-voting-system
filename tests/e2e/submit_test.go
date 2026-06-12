package e2e_test

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
	"github.com/edivilsondalacosta/feature-voting-system/internal/middleware"
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

// memRequestRepo is an in-memory stub for e2e tests.
type memRequestRepo struct {
	requests []*service.FeatureRequest
}

func (m *memRequestRepo) InsertFeatureRequest(_ context.Context, authorID, title, description string) (*service.FeatureRequest, error) {
	fr := &service.FeatureRequest{
		ID:          "test-id",
		AuthorID:    authorID,
		Title:       title,
		Description: description,
		VoteCount:   0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.requests = append(m.requests, fr)
	return fr, nil
}

func (m *memRequestRepo) ListRequestsByNew(_ context.Context, _ service.ListParams) ([]service.FeatureRequest, error) {
	var result []service.FeatureRequest
	for _, r := range m.requests {
		result = append(result, *r)
	}
	return result, nil
}

func (m *memRequestRepo) ListRequestsByTop(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error) {
	return m.ListRequestsByNew(ctx, params)
}

func (m *memRequestRepo) ListRequestsByTrending(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error) {
	return m.ListRequestsByNew(ctx, params)
}

func (m *memRequestRepo) GetFeatureRequest(_ context.Context, id, _ string) (*service.FeatureRequest, error) {
	for _, r := range m.requests {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, service.ErrNotFound
}

func TestSubmitE2E(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh-secret", time.Minute, time.Hour)

	repo := &memRequestRepo{}
	requestSvc := service.NewRequestService(repo, log)
	rankingSvc := service.NewRankingService(repo, log)
	requestsH := handler.NewRequestsHandler(requestSvc, rankingSvc, log)

	r := chi.NewRouter()
	r.Use(middleware.Auth(authSvc))
	r.Post("/requests", requestsH.Submit)
	r.Get("/requests", requestsH.List)

	srv := httptest.NewServer(r)
	defer srv.Close()

	userID := "e2e-user-id"
	token, err := authSvc.IssueAccessToken(userID)
	require.NoError(t, err)

	t.Run("submit_and_verify_vote_count_zero", func(t *testing.T) {
		body := `{"title":"Dark mode","description":"Please add a dark theme."}`
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/requests", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var fr map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&fr))
		assert.Equal(t, float64(0), fr["vote_count"])
		assert.Equal(t, userID, fr["author_id"])
	})

	t.Run("list_returns_etag_header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/requests", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotEmpty(t, resp.Header.Get("ETag"))
	})

	t.Run("conditional_get_304_on_unchanged", func(t *testing.T) {
		// First request to get ETag
		req1, _ := http.NewRequest(http.MethodGet, srv.URL+"/requests", nil)
		req1.Header.Set("Authorization", "Bearer "+token)
		resp1, err := http.DefaultClient.Do(req1)
		require.NoError(t, err)
		resp1.Body.Close()
		etag := resp1.Header.Get("ETag")
		require.NotEmpty(t, etag)

		// Second request with If-None-Match
		req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/requests", nil)
		req2.Header.Set("Authorization", "Bearer "+token)
		req2.Header.Set("If-None-Match", etag)
		resp2, err := http.DefaultClient.Do(req2)
		require.NoError(t, err)
		resp2.Body.Close()
		assert.Equal(t, http.StatusNotModified, resp2.StatusCode)
	})
}
