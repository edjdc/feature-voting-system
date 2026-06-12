package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Healthz returns 200 always (liveness probe).
func Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Health is an alias for liveness (constitution compliance).
func Health(w http.ResponseWriter, r *http.Request) {
	Healthz(w, r)
}

// Readyz checks Postgres + Redis connectivity (readiness probe).
func Readyz(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		checks := map[string]string{}
		status := http.StatusOK

		if err := pool.Ping(ctx); err != nil {
			checks["postgres"] = "unhealthy: " + err.Error()
			status = http.StatusServiceUnavailable
		} else {
			checks["postgres"] = "ok"
		}

		if err := rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "unhealthy: " + err.Error()
			status = http.StatusServiceUnavailable
		} else {
			checks["redis"] = "ok"
		}

		writeJSON(w, status, map[string]any{
			"status": checks,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
