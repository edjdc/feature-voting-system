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

type stubReconcileStore struct {
	getMismatchesFn func(ctx context.Context) ([]struct {
		ID          string
		StoredCount int32
		ActualCount int32
	}, error)
	fixFn       func(ctx context.Context, id string, count int32) error
	setCachedFn func(ctx context.Context, id string, count int32)
}

func (s *stubReconcileStore) GetVoteCountMismatches(ctx context.Context) ([]struct {
	ID          string
	StoredCount int32
	ActualCount int32
}, error) {
	return s.getMismatchesFn(ctx)
}
func (s *stubReconcileStore) FixVoteCount(ctx context.Context, id string, count int32) error {
	return s.fixFn(ctx, id, count)
}
func (s *stubReconcileStore) SetCachedCount(ctx context.Context, id string, count int32) {
	s.setCachedFn(ctx, id, count)
}

func TestReconcileJob_Run_StopsOnContextCancel(t *testing.T) {
	log := observability.NewLogger()
	store := &stubReconcileStore{
		getMismatchesFn: func(_ context.Context) ([]struct {
			ID          string
			StoredCount int32
			ActualCount int32
		}, error) {
			return nil, nil
		},
		fixFn:       func(_ context.Context, _ string, _ int32) error { return nil },
		setCachedFn: func(_ context.Context, _ string, _ int32) {},
	}
	job := ranking.NewReconcileJob(store, 30*time.Millisecond, log)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		job.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ReconcileJob.Run did not stop")
	}
}

func TestReconcileJob_Run_FixesMismatches(t *testing.T) {
	log := observability.NewLogger()

	var fixCallCount atomic.Int32
	store := &stubReconcileStore{
		getMismatchesFn: func(_ context.Context) ([]struct {
			ID          string
			StoredCount int32
			ActualCount int32
		}, error) {
			return []struct {
				ID          string
				StoredCount int32
				ActualCount int32
			}{
				{ID: "r1", StoredCount: 5, ActualCount: 7},
			}, nil
		},
		fixFn: func(_ context.Context, _ string, _ int32) error {
			fixCallCount.Add(1)
			return nil
		},
		setCachedFn: func(_ context.Context, _ string, _ int32) {},
	}
	job := ranking.NewReconcileJob(store, 30*time.Millisecond, log)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Millisecond)
	defer cancel()

	job.Run(ctx)

	assert.GreaterOrEqual(t, int(fixCallCount.Load()), 1)
}

func TestReconcileJob_Run_StoreErrorDoesNotCrash(t *testing.T) {
	log := observability.NewLogger()
	store := &stubReconcileStore{
		getMismatchesFn: func(_ context.Context) ([]struct {
			ID          string
			StoredCount int32
			ActualCount int32
		}, error) {
			return nil, errors.New("db error")
		},
		fixFn:       func(_ context.Context, _ string, _ int32) error { return nil },
		setCachedFn: func(_ context.Context, _ string, _ int32) {},
	}
	job := ranking.NewReconcileJob(store, 30*time.Millisecond, log)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	job.Run(ctx) // must not panic
}
