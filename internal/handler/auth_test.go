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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/handler"
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type stubUserRepo struct {
	createFn func(ctx context.Context, email, passwordHash string) (service.UserRecord, error)
	getFn    func(ctx context.Context, email string) (service.UserRecord, error)
}

func (s *stubUserRepo) CreateUser(ctx context.Context, email, passwordHash string) (service.UserRecord, error) {
	return s.createFn(ctx, email, passwordHash)
}
func (s *stubUserRepo) GetUserByEmail(ctx context.Context, email string) (service.UserRecord, error) {
	return s.getFn(ctx, email)
}

func TestAuthHandler_Register_Success(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	repo := &stubUserRepo{
		createFn: func(_ context.Context, email, _ string) (service.UserRecord, error) {
			return service.UserRecord{ID: "user-1", Email: email}, nil
		},
	}
	userSvc := service.NewUserService(repo, authSvc, log)
	h := handler.NewAuthHandler(authSvc, userSvc, log)

	body := `{"email":"alice@example.com","password":"pass"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Register(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp["access_token"])
	assert.NotEmpty(t, resp["refresh_token"])
}

func TestAuthHandler_Register_MissingFields(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	userSvc := service.NewUserService(&stubUserRepo{}, authSvc, log)
	h := handler.NewAuthHandler(authSvc, userSvc, log)

	body := `{"email":"","password":""}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.Register(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_Register_EmailTaken(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	repo := &stubUserRepo{
		createFn: func(_ context.Context, _, _ string) (service.UserRecord, error) {
			return service.UserRecord{}, errors.New("duplicate key value violates unique constraint")
		},
	}
	userSvc := service.NewUserService(repo, authSvc, log)
	h := handler.NewAuthHandler(authSvc, userSvc, log)

	body := `{"email":"alice@example.com","password":"pass"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Register(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAuthHandler_Login_Success(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	// Pre-hash a password
	hash, _ := authSvc.HashPassword("correct-password")
	repo := &stubUserRepo{
		getFn: func(_ context.Context, email string) (service.UserRecord, error) {
			return service.UserRecord{ID: "user-1", Email: email, PasswordHash: hash}, nil
		},
	}
	userSvc := service.NewUserService(repo, authSvc, log)
	h := handler.NewAuthHandler(authSvc, userSvc, log)

	body := `{"email":"alice@example.com","password":"correct-password"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Login(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp["access_token"])
}

func TestAuthHandler_Login_WrongPassword(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	hash, _ := authSvc.HashPassword("correct")
	repo := &stubUserRepo{
		getFn: func(_ context.Context, email string) (service.UserRecord, error) {
			return service.UserRecord{ID: "user-1", Email: email, PasswordHash: hash}, nil
		},
	}
	userSvc := service.NewUserService(repo, authSvc, log)
	h := handler.NewAuthHandler(authSvc, userSvc, log)

	body := `{"email":"alice@example.com","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Login(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Refresh_Success(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	userSvc := service.NewUserService(&stubUserRepo{}, authSvc, log)
	h := handler.NewAuthHandler(authSvc, userSvc, log)

	refreshToken, _ := authSvc.IssueRefreshToken("user-1")
	body, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Refresh(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp["access_token"])
}

func TestAuthHandler_Refresh_InvalidToken(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	userSvc := service.NewUserService(&stubUserRepo{}, authSvc, log)
	h := handler.NewAuthHandler(authSvc, userSvc, log)

	body := `{"refresh_token":"invalid.token.value"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.Refresh(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
