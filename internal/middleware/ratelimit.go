package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimit implements a token-bucket rate limiter using Redis.
// limit: max requests per window, window: time window duration.
func RateLimit(rdb *redis.Client, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := GetUserID(r)
			if !ok {
				// fallback to IP-based rate limiting
				userID = r.RemoteAddr
			}

			windowSecs := int(window.Seconds())
			key := fmt.Sprintf("ratelimit:%s:%d", userID, time.Now().Unix()/int64(windowSecs))

			count, err := rdb.Incr(r.Context(), key).Result()
			if err != nil {
				// fail open — don't block on Redis errors
				next.ServeHTTP(w, r)
				return
			}

			if count == 1 {
				rdb.Expire(r.Context(), key, window)
			}

			if count > int64(limit) {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", windowSecs))
				http.Error(w, `{"error":"rate_limited","message":"too many requests"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
