package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type AuthHandler struct {
	auth    *service.AuthService
	userSvc *service.UserService
	log     *slog.Logger
}

func NewAuthHandler(auth *service.AuthService, userSvc *service.UserService, log *slog.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, userSvc: userSvc, log: log}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}

// Register creates a new user account.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "email and password are required")
		return
	}

	user, err := h.userSvc.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			writeError(w, http.StatusConflict, "email_taken", "email already registered")
			return
		}
		h.log.Error("register user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "registration failed")
		return
	}

	accessToken, err := h.auth.IssueAccessToken(user.ID)
	if err != nil {
		h.log.Error("issue access token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "token issuance failed")
		return
	}
	refreshToken, err := h.auth.IssueRefreshToken(user.ID)
	if err != nil {
		h.log.Error("issue refresh token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "token issuance failed")
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// Login authenticates an existing user.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	user, err := h.userSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "email or password incorrect")
			return
		}
		h.log.Error("login user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "login failed")
		return
	}

	accessToken, _ := h.auth.IssueAccessToken(user.ID)
	refreshToken, _ := h.auth.IssueRefreshToken(user.ID)

	writeJSON(w, http.StatusOK, authResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// Refresh issues a new access token from a valid refresh token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	claims, err := h.auth.VerifyRefreshToken(req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "refresh token invalid or expired")
		return
	}

	accessToken, err := h.auth.IssueAccessToken(claims.UserID)
	if err != nil {
		h.log.Error("issue access token on refresh", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "token issuance failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"access_token": accessToken,
	})
}
