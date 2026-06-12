package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	pgRepo "github.com/edivilsondalacosta/feature-voting-system/internal/repository/postgres"
	"github.com/edivilsondalacosta/feature-voting-system/internal/ranking"
)

// trendingStoreAdapter wraps the postgres ranking repo + redis cache to implement ranking.TrendingStore.
type trendingStoreAdapter struct {
	pg  *pgRepo.RankingRepo
	rdb *redis.Client
}

func (a *trendingStoreAdapter) GetAllRequests(ctx context.Context) ([]ranking.RequestForScoring, error) {
	return a.pg.GetAllRequests(ctx)
}

func (a *trendingStoreAdapter) SetTrendingScore(ctx context.Context, requestID string, score float64) error {
	return a.rdb.ZAdd(ctx, "trending", redis.Z{Score: score, Member: requestID}).Err()
}

// reconcileStoreAdapter wraps postgres + redis for the reconcile job.
type reconcileStoreAdapter struct {
	pg  *pgRepo.RankingRepo
	rdb *redis.Client
}

func (a *reconcileStoreAdapter) GetVoteCountMismatches(ctx context.Context) ([]struct {
	ID          string
	StoredCount int32
	ActualCount int32
}, error) {
	return a.pg.GetVoteCountMismatches(ctx)
}

func (a *reconcileStoreAdapter) FixVoteCount(ctx context.Context, requestID string, count int32) error {
	return a.pg.FixVoteCount(ctx, requestID, count)
}

func (a *reconcileStoreAdapter) SetCachedCount(ctx context.Context, requestID string, count int32) {
	key := "count:" + requestID
	_ = a.rdb.Set(ctx, key, count, 0).Err()
}

func connectRedis(ctx context.Context, url string, log *slog.Logger) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		log.Warn("redis not reachable at startup, will retry on requests", "error", err)
	}
	return client, nil
}
