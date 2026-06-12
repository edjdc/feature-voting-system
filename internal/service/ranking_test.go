package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type stubRankingRepo struct {
	topFn      func(ctx context.Context, p service.ListParams) ([]service.FeatureRequest, error)
	trendingFn func(ctx context.Context, p service.ListParams) ([]service.FeatureRequest, error)
	newFn      func(ctx context.Context, p service.ListParams) ([]service.FeatureRequest, error)
}

func (s *stubRankingRepo) ListRequestsByTop(ctx context.Context, p service.ListParams) ([]service.FeatureRequest, error) {
	return s.topFn(ctx, p)
}
func (s *stubRankingRepo) ListRequestsByTrending(ctx context.Context, p service.ListParams) ([]service.FeatureRequest, error) {
	return s.trendingFn(ctx, p)
}
func (s *stubRankingRepo) ListRequestsByNew(ctx context.Context, p service.ListParams) ([]service.FeatureRequest, error) {
	return s.newFn(ctx, p)
}

func makeFRs(n int) []service.FeatureRequest {
	frs := make([]service.FeatureRequest, n)
	for i := range frs {
		frs[i] = service.FeatureRequest{
			ID:        "id",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}
	return frs
}

func TestRankingService_List_SortNew(t *testing.T) {
	log := observability.NewLogger()
	repo := &stubRankingRepo{
		newFn: func(_ context.Context, _ service.ListParams) ([]service.FeatureRequest, error) {
			return makeFRs(3), nil
		},
	}
	svc := service.NewRankingService(repo, log)

	page, err := svc.List(context.Background(), service.ListParams{Sort: "new", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, page.Items, 3)
	assert.Nil(t, page.NextCursor)
}

func TestRankingService_List_SortTop(t *testing.T) {
	log := observability.NewLogger()
	repo := &stubRankingRepo{
		topFn: func(_ context.Context, _ service.ListParams) ([]service.FeatureRequest, error) {
			return makeFRs(5), nil
		},
	}
	svc := service.NewRankingService(repo, log)

	page, err := svc.List(context.Background(), service.ListParams{Sort: "top", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, page.Items, 5)
}

func TestRankingService_List_SortTrending(t *testing.T) {
	log := observability.NewLogger()
	repo := &stubRankingRepo{
		trendingFn: func(_ context.Context, _ service.ListParams) ([]service.FeatureRequest, error) {
			return makeFRs(2), nil
		},
	}
	svc := service.NewRankingService(repo, log)

	page, err := svc.List(context.Background(), service.ListParams{Sort: "trending", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, page.Items, 2)
}

func TestRankingService_List_Pagination(t *testing.T) {
	log := observability.NewLogger()
	repo := &stubRankingRepo{
		newFn: func(_ context.Context, p service.ListParams) ([]service.FeatureRequest, error) {
			// Return limit+1 to signal there's a next page.
			return makeFRs(p.Limit), nil
		},
	}
	svc := service.NewRankingService(repo, log)

	page, err := svc.List(context.Background(), service.ListParams{Sort: "new", Limit: 5})
	require.NoError(t, err)
	assert.Len(t, page.Items, 5)
	assert.NotNil(t, page.NextCursor)
}

func TestRankingService_List_DefaultLimit(t *testing.T) {
	log := observability.NewLogger()
	var capturedLimit int
	repo := &stubRankingRepo{
		newFn: func(_ context.Context, p service.ListParams) ([]service.FeatureRequest, error) {
			capturedLimit = p.Limit
			return nil, nil
		},
	}
	svc := service.NewRankingService(repo, log)

	_, err := svc.List(context.Background(), service.ListParams{Sort: "new", Limit: 0})
	require.NoError(t, err)
	// Default limit is 20, so fetch limit is 21.
	assert.Equal(t, 21, capturedLimit)
}

func TestRankingService_List_ExcessiveLimitClamped(t *testing.T) {
	log := observability.NewLogger()
	var capturedLimit int
	repo := &stubRankingRepo{
		newFn: func(_ context.Context, p service.ListParams) ([]service.FeatureRequest, error) {
			capturedLimit = p.Limit
			return nil, nil
		},
	}
	svc := service.NewRankingService(repo, log)

	_, err := svc.List(context.Background(), service.ListParams{Sort: "new", Limit: 999})
	require.NoError(t, err)
	// Clamped to 20, fetch 21.
	assert.Equal(t, 21, capturedLimit)
}
