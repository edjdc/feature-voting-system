package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func Auth(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"unauthorized","message":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				http.Error(w, `{"error":"unauthorized","message":"invalid authorization header format"}`, http.StatusUnauthorized)
				return
			}

			claims, err := authSvc.VerifyAccessToken(parts[1])
			if err != nil {
				http.Error(w, `{"error":"unauthorized","message":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(r *http.Request) (string, bool) {
	id, ok := r.Context().Value(UserIDKey).(string)
	return id, ok && id != ""
}
