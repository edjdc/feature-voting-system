package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VoteRepo struct {
	pool *pgxpool.Pool
}

func NewVoteRepo(pool *pgxpool.Pool) *VoteRepo {
	return &VoteRepo{pool: pool}
}

// InsertVote atomically inserts a vote and increments vote_count.
// Returns inserted=true if a new vote was created (false if already existed).
func (r *VoteRepo) InsertVote(ctx context.Context, userID, requestID string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertVoteSQL = `
INSERT INTO votes (user_id, request_id) VALUES ($1, $2)
ON CONFLICT (user_id, request_id) DO NOTHING`

	ct, err := tx.Exec(ctx, insertVoteSQL, userID, requestID)
	if err != nil {
		return false, fmt.Errorf("insert vote: %w", err)
	}

	inserted := ct.RowsAffected() > 0

	if inserted {
		const incrSQL = `
UPDATE feature_requests SET vote_count = vote_count + 1, updated_at = now()
WHERE id = $1`
		if _, err := tx.Exec(ctx, incrSQL, requestID); err != nil {
			return false, fmt.Errorf("increment vote_count: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit tx: %w", err)
	}
	return inserted, nil
}

// DeleteVote atomically deletes a vote and decrements vote_count.
// Returns deleted=true if the vote existed.
func (r *VoteRepo) DeleteVote(ctx context.Context, userID, requestID string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const deleteVoteSQL = `DELETE FROM votes WHERE user_id = $1 AND request_id = $2`

	ct, err := tx.Exec(ctx, deleteVoteSQL, userID, requestID)
	if err != nil {
		return false, fmt.Errorf("delete vote: %w", err)
	}

	deleted := ct.RowsAffected() > 0

	if deleted {
		const decrSQL = `
UPDATE feature_requests SET vote_count = GREATEST(vote_count - 1, 0), updated_at = now()
WHERE id = $1`
		if _, err := tx.Exec(ctx, decrSQL, requestID); err != nil {
			return false, fmt.Errorf("decrement vote_count: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit tx: %w", err)
	}
	return deleted, nil
}

func (r *VoteRepo) GetVoteCount(ctx context.Context, requestID string) (int32, error) {
	const q = `SELECT vote_count FROM feature_requests WHERE id = $1`
	row := r.pool.QueryRow(ctx, q, requestID)
	var count int32
	if err := row.Scan(&count); err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("feature request not found")
		}
		return 0, fmt.Errorf("get vote count: %w", err)
	}
	return count, nil
}

func (r *VoteRepo) GetRequestAuthorID(ctx context.Context, requestID string) (string, error) {
	const q = `SELECT author_id FROM feature_requests WHERE id = $1`
	row := r.pool.QueryRow(ctx, q, requestID)
	var authorID pgtype.UUID
	if err := row.Scan(&authorID); err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("feature request not found")
		}
		return "", fmt.Errorf("get author id: %w", err)
	}
	return authorID.String(), nil
}
