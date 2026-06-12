package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type VoteCache struct {
	rdb *redis.Client
}

func NewVoteCache(rdb *redis.Client) *VoteCache {
	return &VoteCache{rdb: rdb}
}

func countKey(requestID string) string {
	return "count:" + requestID
}

// IncrementCount updates the cached vote count and the top ZSET best-effort.
func (c *VoteCache) IncrementCount(ctx context.Context, requestID string, newCount int32) {
	pipe := c.rdb.Pipeline()
	pipe.Set(ctx, countKey(requestID), newCount, 0)
	pipe.ZAdd(ctx, "top", redis.Z{Score: float64(newCount), Member: requestID})
	_, _ = pipe.Exec(ctx) // best-effort; errors are non-fatal
}

// DecrementCount updates the cached vote count and the top ZSET best-effort.
func (c *VoteCache) DecrementCount(ctx context.Context, requestID string, newCount int32) {
	pipe := c.rdb.Pipeline()
	pipe.Set(ctx, countKey(requestID), newCount, 0)
	pipe.ZAdd(ctx, "top", redis.Z{Score: float64(newCount), Member: requestID})
	_, _ = pipe.Exec(ctx)
}

// GetCount returns the cached vote count; returns -1 if not cached.
func (c *VoteCache) GetCount(ctx context.Context, requestID string) int32 {
	val, err := c.rdb.Get(ctx, countKey(requestID)).Result()
	if err != nil {
		return -1
	}
	n, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return -1
	}
	return int32(n)
}

// SetTrendingScore updates the trending ZSET.
func (c *VoteCache) SetTrendingScore(ctx context.Context, requestID string, score float64) error {
	return c.rdb.ZAdd(ctx, "trending", redis.Z{Score: score, Member: requestID}).Err()
}

// GetTopRanked returns the top N request IDs from the Redis ZSET.
func (c *VoteCache) GetTopRanked(ctx context.Context, n int64) ([]string, error) {
	result, err := c.rdb.ZRevRangeByScore(ctx, "top", &redis.ZRangeBy{
		Max:    "+inf",
		Min:    "-inf",
		Offset: 0,
		Count:  n,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("get top ranked: %w", err)
	}
	return result, nil
}

// GetTrendingRanked returns the top N request IDs from the trending ZSET.
func (c *VoteCache) GetTrendingRanked(ctx context.Context, n int64) ([]string, error) {
	result, err := c.rdb.ZRevRangeByScore(ctx, "trending", &redis.ZRangeBy{
		Max:    "+inf",
		Min:    "-inf",
		Offset: 0,
		Count:  n,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("get trending ranked: %w", err)
	}
	return result, nil
}
