package ranking

import (
	"context"
	"log/slog"
	"math"
	"time"
)

// TrendingScore computes the trending score for a feature request.
// Formula: vote_count / (age_hours + 2)^1.5
func TrendingScore(voteCount int32, createdAt time.Time) float64 {
	ageHours := time.Since(createdAt).Hours()
	return float64(voteCount) / math.Pow(ageHours+2, 1.5)
}

type RequestForScoring struct {
	ID        string
	VoteCount int32
	CreatedAt time.Time
}

type TrendingStore interface {
	GetAllRequests(ctx context.Context) ([]RequestForScoring, error)
	SetTrendingScore(ctx context.Context, requestID string, score float64) error
}

type TrendingJob struct {
	store    TrendingStore
	interval time.Duration
	log      *slog.Logger
}

func NewTrendingJob(store TrendingStore, interval time.Duration, log *slog.Logger) *TrendingJob {
	return &TrendingJob{store: store, interval: interval, log: log}
}

// Run starts the periodic trending recompute job. It stops when ctx is cancelled.
func (j *TrendingJob) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			j.log.Info("trending job stopping")
			return
		case <-ticker.C:
			if err := j.recompute(ctx); err != nil {
				j.log.Error("trending recompute failed", "error", err)
			}
		}
	}
}

func (j *TrendingJob) recompute(ctx context.Context) error {
	requests, err := j.store.GetAllRequests(ctx)
	if err != nil {
		return err
	}

	for _, req := range requests {
		score := TrendingScore(req.VoteCount, req.CreatedAt)
		if err := j.store.SetTrendingScore(ctx, req.ID, score); err != nil {
			j.log.Error("set trending score", "request_id", req.ID, "error", err)
		}
	}
	return nil
}
