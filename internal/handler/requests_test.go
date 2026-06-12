package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type stubRequestRepo struct {
	insertFn func(ctx context.Context, authorID, title, description string) (*service.FeatureRequest, error)
	listNewFn func(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error)
	listTopFn func(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error)
	listTrendingFn func(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error)
	getFn     func(ctx context.Context, id, viewerID string) (*service.FeatureRequest, error)
}

func (s *stubRequestRepo) InsertFeatureRequest(ctx context.Context, authorID, title, description string) (*service.FeatureRequest, error) {
	return s.insertFn(ctx, authorID, title, description)
}
func (s *stubRequestRepo) ListRequestsByNew(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error) {
	return s.listNewFn(ctx, params)
}
func (s *stubRequestRepo) ListRequestsByTop(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error) {
	return s.listTopFn(ctx, params)
}
func (s *stubRequestRepo) ListRequestsByTrending(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error) {
	return s.listTrendingFn(ctx, params)
}
func (s *stubRequestRepo) GetFeatureRequest(ctx context.Context, id, viewerID string) (*service.FeatureRequest, error) {
	return s.getFn(ctx, id, viewerID)
}

var fixedTime = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

func sampleFR(authorID string) *service.FeatureRequest {
	return &service.FeatureRequest{
		ID:          "fr-123",
		AuthorID:    authorID,
		Title:       "Dark mode",
		Description: "Please add dark theme",
		VoteCount:   0,
		CreatedAt:   fixedTime,
		UpdatedAt:   fixedTime,
	}
}

func makeHandlers(insertFn func(context.Context, string, string, string) (*service.FeatureRequest, error)) (*handler.RequestsHandler, *handler.RequestsHandler) {
	log := observability.NewLogger()
	repo := &stubRequestRepo{
		insertFn: insertFn,
		listNewFn: func(_ context.Context, p service.ListParams) ([]service.FeatureRequest, error) {
			return []service.FeatureRequest{*sampleFR("user-1")}, nil
		},
		listTopFn: func(_ context.Context, p service.ListParams) ([]service.FeatureRequest, error) {
			return []service.FeatureRequest{*sampleFR("user-1")}, nil
		},
		listTrendingFn: func(_ context.Context, p service.ListParams) ([]service.FeatureRequest, error) {
			return []service.FeatureRequest{*sampleFR("user-1")}, nil
		},
		getFn: func(_ context.Context, id, _ string) (*service.FeatureRequest, error) {
			if id == "fr-123" {
				return sampleFR("user-1"), nil
			}
			return nil, service.ErrNotFound
		},
	}
	requestSvc := service.NewRequestService(repo, log)
	rankingSvc := service.NewRankingService(repo, log)
	h := handler.NewRequestsHandler(requestSvc, rankingSvc, log)
	return h, h
}

func withUserIDCtx(r *http.Request, userID string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, userID))
}

func TestRequestsHandler_Submit_Valid(t *testing.T) {
	h, _ := makeHandlers(func(_ context.Context, authorID, title, description string) (*service.FeatureRequest, error) {
		return sampleFR(authorID), nil
	})

	body := `{"title":"Dark mode","description":"Please add dark theme."}`
	req := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserIDCtx(req, "user-1")
	w := httptest.NewRecorder()
	h.Submit(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var fr map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&fr))
	assert.Equal(t, float64(0), fr["vote_count"])
}

func TestRequestsHandler_Submit_NoAuth(t *testing.T) {
	h, _ := makeHandlers(nil)
	body := `{"title":"Dark mode","description":"Please add dark theme."}`
	req := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.Submit(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequestsHandler_Submit_ValidationError(t *testing.T) {
	h, _ := makeHandlers(nil)
	body := `{"title":"","description":"Some desc"}`
	req := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewBufferString(body))
	req = withUserIDCtx(req, "user-1")
	w := httptest.NewRecorder()
	h.Submit(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "validation_failed", resp["error"])
}

func TestRequestsHandler_List_ReturnsETag(t *testing.T) {
	h, _ := makeHandlers(nil)
	req := httptest.NewRequest(http.MethodGet, "/requests?sort=new&limit=10", nil)
	req = withUserIDCtx(req, "user-1")
	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("ETag"))
	assert.Equal(t, "private, max-age=0, must-revalidate", w.Header().Get("Cache-Control"))
}

func TestRequestsHandler_List_304OnUnchanged(t *testing.T) {
	h, _ := makeHandlers(nil)

	req1 := httptest.NewRequest(http.MethodGet, "/requests?sort=new", nil)
	req1 = withUserIDCtx(req1, "user-1")
	w1 := httptest.NewRecorder()
	h.List(w1, req1)
	etag := w1.Header().Get("ETag")
	require.NotEmpty(t, etag)

	req2 := httptest.NewRequest(http.MethodGet, "/requests?sort=new", nil)
	req2 = withUserIDCtx(req2, "user-1")
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	h.List(w2, req2)
	assert.Equal(t, http.StatusNotModified, w2.Code)
}

func TestRequestsHandler_GetByID_Found(t *testing.T) {
	h, _ := makeHandlers(nil)

	r := chi.NewRouter()
	r.Get("/requests/{id}", h.GetByID)
	srv := httptest.NewServer(r)
	defer srv.Close()

	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	token, _ := authSvc.IssueAccessToken("user-1")

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/requests/fr-123", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Add middleware
	r2 := chi.NewRouter()
	r2.Use(middleware.Auth(authSvc))
	r2.Get("/requests/{id}", h.GetByID)
	srv2 := httptest.NewServer(r2)
	defer srv2.Close()

	req2, _ := http.NewRequest(http.MethodGet, srv2.URL+"/requests/fr-123", nil)
	req2.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRequestsHandler_GetByID_NotFound(t *testing.T) {
	log := observability.NewLogger()
	repo := &stubRequestRepo{
		getFn: func(_ context.Context, _, _ string) (*service.FeatureRequest, error) {
			return nil, service.ErrNotFound
		},
		listNewFn: func(_ context.Context, _ service.ListParams) ([]service.FeatureRequest, error) {
			return nil, nil
		},
		listTopFn: func(_ context.Context, _ service.ListParams) ([]service.FeatureRequest, error) {
			return nil, nil
		},
		listTrendingFn: func(_ context.Context, _ service.ListParams) ([]service.FeatureRequest, error) {
			return nil, nil
		},
	}
	requestSvc := service.NewRequestService(repo, log)
	rankingSvc := service.NewRankingService(repo, log)
	h := handler.NewRequestsHandler(requestSvc, rankingSvc, log)

	r := chi.NewRouter()
	r.Use(middleware.Auth(service.NewAuthService("secret", "refresh", time.Minute, time.Hour)))
	r.Get("/requests/{id}", h.GetByID)
	srv := httptest.NewServer(r)
	defer srv.Close()

	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	token, _ := authSvc.IssueAccessToken("user-1")

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/requests/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRequestsHandler_Submit_RepoError(t *testing.T) {
	h, _ := makeHandlers(func(_ context.Context, _, _, _ string) (*service.FeatureRequest, error) {
		return nil, errors.New("db error")
	})
	body := `{"title":"Dark mode","description":"Please add dark theme."}`
	req := httptest.NewRequest(http.MethodPost, "/requests", bytes.NewBufferString(body))
	req = withUserIDCtx(req, "user-1")
	w := httptest.NewRecorder()
	h.Submit(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
