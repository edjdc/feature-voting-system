package handler_test

import (
	"context"
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

type stubVoteRepo struct {
	authorFn  func(ctx context.Context, requestID string) (string, error)
	insertFn  func(ctx context.Context, userID, requestID string) (bool, error)
	deleteFn  func(ctx context.Context, userID, requestID string) (bool, error)
	countFn   func(ctx context.Context, requestID string) (int32, error)
}

func (s *stubVoteRepo) GetRequestAuthorID(ctx context.Context, requestID string) (string, error) {
	return s.authorFn(ctx, requestID)
}
func (s *stubVoteRepo) InsertVote(ctx context.Context, userID, requestID string) (bool, error) {
	return s.insertFn(ctx, userID, requestID)
}
func (s *stubVoteRepo) DeleteVote(ctx context.Context, userID, requestID string) (bool, error) {
	return s.deleteFn(ctx, userID, requestID)
}
func (s *stubVoteRepo) GetVoteCount(ctx context.Context, requestID string) (int32, error) {
	return s.countFn(ctx, requestID)
}

func voteRouter(voteSvc *service.VoteService, authSvc *service.AuthService) *chi.Mux {
	log := observability.NewLogger()
	h := handler.NewVotesHandler(voteSvc, log)
	r := chi.NewRouter()
	r.Use(middleware.Auth(authSvc))
	r.Put("/requests/{id}/vote", h.Upvote)
	r.Delete("/requests/{id}/vote", h.RemoveVote)
	return r
}

func TestVotesHandler_Upvote_Success(t *testing.T) {
	repo := &stubVoteRepo{
		authorFn:  func(_ context.Context, _ string) (string, error) { return "alice", nil },
		insertFn:  func(_ context.Context, _, _ string) (bool, error) { return true, nil },
		countFn:   func(_ context.Context, _ string) (int32, error) { return 1, nil },
	}
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	voteSvc := service.NewVoteService(repo, observability.NewLogger())
	srv := httptest.NewServer(voteRouter(voteSvc, authSvc))
	defer srv.Close()

	token, _ := authSvc.IssueAccessToken("bob")
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/requests/req-1/vote", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestVotesHandler_Upvote_SelfVote_Returns403(t *testing.T) {
	repo := &stubVoteRepo{
		authorFn: func(_ context.Context, _ string) (string, error) { return "alice", nil },
	}
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	voteSvc := service.NewVoteService(repo, observability.NewLogger())
	srv := httptest.NewServer(voteRouter(voteSvc, authSvc))
	defer srv.Close()

	token, _ := authSvc.IssueAccessToken("alice") // voting on own request
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/requests/req-1/vote", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestVotesHandler_Upvote_NotFound_Returns404(t *testing.T) {
	repo := &stubVoteRepo{
		authorFn: func(_ context.Context, _ string) (string, error) {
			return "", service.ErrNotFound
		},
	}
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	voteSvc := service.NewVoteService(repo, observability.NewLogger())
	srv := httptest.NewServer(voteRouter(voteSvc, authSvc))
	defer srv.Close()

	token, _ := authSvc.IssueAccessToken("bob")
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/requests/nonexistent/vote", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestVotesHandler_RemoveVote_Success(t *testing.T) {
	repo := &stubVoteRepo{
		authorFn: func(_ context.Context, _ string) (string, error) { return "alice", nil },
		deleteFn: func(_ context.Context, _, _ string) (bool, error) { return true, nil },
		countFn:  func(_ context.Context, _ string) (int32, error) { return 0, nil },
	}
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	voteSvc := service.NewVoteService(repo, observability.NewLogger())
	srv := httptest.NewServer(voteRouter(voteSvc, authSvc))
	defer srv.Close()

	token, _ := authSvc.IssueAccessToken("bob")
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/requests/req-1/vote", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestVotesHandler_NoAuth_Returns401(t *testing.T) {
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	voteSvc := service.NewVoteService(&stubVoteRepo{}, observability.NewLogger())
	srv := httptest.NewServer(voteRouter(voteSvc, authSvc))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/requests/req-1/vote", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
