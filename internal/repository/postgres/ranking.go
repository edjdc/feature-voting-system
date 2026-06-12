package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edivilsondalacosta/feature-voting-system/internal/ranking"
)

type RankingRepo struct {
	pool *pgxpool.Pool
}

func NewRankingRepo(pool *pgxpool.Pool) *RankingRepo {
	return &RankingRepo{pool: pool}
}

func (r *RankingRepo) GetAllRequests(ctx context.Context) ([]ranking.RequestForScoring, error) {
	const q = `SELECT id, vote_count, created_at FROM feature_requests`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get all requests: %w", err)
	}
	defer rows.Close()

	var result []ranking.RequestForScoring
	for rows.Next() {
		var id pgtype.UUID
		var voteCount int32
		var createdAt pgtype.Timestamptz
		if err := rows.Scan(&id, &voteCount, &createdAt); err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		result = append(result, ranking.RequestForScoring{
			ID:        id.String(),
			VoteCount: voteCount,
			CreatedAt: createdAt.Time,
		})
	}
	return result, rows.Err()
}

func (r *RankingRepo) GetVoteCountMismatches(ctx context.Context) ([]struct {
	ID          string
	StoredCount int32
	ActualCount int32
}, error) {
	const q = `
SELECT fr.id, fr.vote_count AS stored_count, COUNT(v.user_id)::INTEGER AS actual_count
FROM feature_requests fr
LEFT JOIN votes v ON v.request_id = fr.id
GROUP BY fr.id, fr.vote_count
HAVING fr.vote_count != COUNT(v.user_id)`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get mismatches: %w", err)
	}
	defer rows.Close()

	var result []struct {
		ID          string
		StoredCount int32
		ActualCount int32
	}
	for rows.Next() {
		var id pgtype.UUID
		var stored, actual int32
		if err := rows.Scan(&id, &stored, &actual); err != nil {
			return nil, fmt.Errorf("scan mismatch: %w", err)
		}
		result = append(result, struct {
			ID          string
			StoredCount int32
			ActualCount int32
		}{ID: id.String(), StoredCount: stored, ActualCount: actual})
	}
	return result, rows.Err()
}

func (r *RankingRepo) FixVoteCount(ctx context.Context, requestID string, count int32) error {
	const q = `UPDATE feature_requests SET vote_count = $1, updated_at = now() WHERE id = $2`
	_, err := r.pool.Exec(ctx, q, count, requestID)
	if err != nil {
		return fmt.Errorf("fix vote count: %w", err)
	}
	return nil
}
