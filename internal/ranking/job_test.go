package ranking_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	"github.com/edivilsondalacosta/feature-voting-system/internal/ranking"
)

type stubTrendingStore struct {
	getAllFn  func(ctx context.Context) ([]ranking.RequestForScoring, error)
	setFn    func(ctx context.Context, id string, score float64) error
}

func (s *stubTrendingStore) GetAllRequests(ctx context.Context) ([]ranking.RequestForScoring, error) {
	return s.getAllFn(ctx)
}
func (s *stubTrendingStore) SetTrendingScore(ctx context.Context, id string, score float64) error {
	return s.setFn(ctx, id, score)
}

func TestTrendingJob_Run_StopsOnContextCancel(t *testing.T) {
	log := observability.NewLogger()
	store := &stubTrendingStore{
		getAllFn: func(_ context.Context) ([]ranking.RequestForScoring, error) {
			return nil, nil
		},
		setFn: func(_ context.Context, _ string, _ float64) error { return nil },
	}
	job := ranking.NewTrendingJob(store, 50*time.Millisecond, log)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		job.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// ok — job stopped when ctx was cancelled
	case <-time.After(500 * time.Millisecond):
		t.Fatal("TrendingJob.Run did not stop after context cancellation")
	}
}

func TestTrendingJob_Run_ComputesScores(t *testing.T) {
	log := observability.NewLogger()

	var scoreCallCount atomic.Int32
	store := &stubTrendingStore{
		getAllFn: func(_ context.Context) ([]ranking.RequestForScoring, error) {
			return []ranking.RequestForScoring{
				{ID: "r1", VoteCount: 10, CreatedAt: time.Now().Add(-2 * time.Hour)},
				{ID: "r2", VoteCount: 5, CreatedAt: time.Now().Add(-1 * time.Hour)},
			}, nil
		},
		setFn: func(_ context.Context, _ string, _ float64) error {
			scoreCallCount.Add(1)
			return nil
		},
	}
	job := ranking.NewTrendingJob(store, 30*time.Millisecond, log)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	job.Run(ctx)

	assert.GreaterOrEqual(t, int(scoreCallCount.Load()), 2, "expected at least one recompute cycle")
}

func TestTrendingJob_Run_StoreErrorDoesNotCrash(t *testing.T) {
	log := observability.NewLogger()
	store := &stubTrendingStore{
		getAllFn: func(_ context.Context) ([]ranking.RequestForScoring, error) {
			return nil, errors.New("db unavailable")
		},
		setFn: func(_ context.Context, _ string, _ float64) error { return nil },
	}
	job := ranking.NewTrendingJob(store, 30*time.Millisecond, log)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	// Should not panic on store errors.
	job.Run(ctx)
}
