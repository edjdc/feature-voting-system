package postgres

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type RequestRepo struct {
	pool *pgxpool.Pool
}

func NewRequestRepo(pool *pgxpool.Pool) *RequestRepo {
	return &RequestRepo{pool: pool}
}

const insertFeatureRequestSQL = `
INSERT INTO feature_requests (author_id, title, description)
VALUES ($1, $2, $3)
RETURNING id, author_id, title, description, vote_count, created_at, updated_at`

func (r *RequestRepo) InsertFeatureRequest(ctx context.Context, authorID, title, description string) (*service.FeatureRequest, error) {
	row := r.pool.QueryRow(ctx, insertFeatureRequestSQL, authorID, title, description)
	return scanRequest(row)
}

func (r *RequestRepo) GetFeatureRequest(ctx context.Context, id, viewerID string) (*service.FeatureRequest, error) {
	const q = `
SELECT fr.id, fr.author_id, fr.title, fr.description, fr.vote_count, fr.created_at, fr.updated_at,
       EXISTS(SELECT 1 FROM votes v WHERE v.request_id = fr.id AND v.user_id = $2) AS viewer_has_voted
FROM feature_requests fr
WHERE fr.id = $1`

	row := r.pool.QueryRow(ctx, q, id, viewerID)
	return scanRequestWithVote(row)
}

func (r *RequestRepo) ListRequestsByNew(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error) {
	var sb strings.Builder
	args := []any{}
	argIdx := 1

	sb.WriteString(`
SELECT fr.id, fr.author_id, fr.title, fr.description, fr.vote_count, fr.created_at, fr.updated_at,
       EXISTS(SELECT 1 FROM votes v WHERE v.request_id = fr.id AND v.user_id = $`)
	sb.WriteString(fmt.Sprintf("%d", argIdx))
	sb.WriteString(`) AS viewer_has_voted FROM feature_requests fr`)
	args = append(args, params.ViewerID)
	argIdx++

	if params.Cursor != nil && *params.Cursor != "" {
		cur, err := decodeCursorRaw(*params.Cursor)
		if err == nil {
			sb.WriteString(fmt.Sprintf(` WHERE (fr.created_at, fr.id) < ($%d, $%d)`, argIdx, argIdx+1))
			args = append(args, cur.createdAt, cur.id)
			argIdx += 2
		}
	}

	sb.WriteString(fmt.Sprintf(` ORDER BY fr.created_at DESC, fr.id DESC LIMIT $%d`, argIdx))
	args = append(args, params.Limit)

	rows, err := r.pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list by new: %w", err)
	}
	defer rows.Close()
	return collectRequests(rows)
}

func (r *RequestRepo) ListRequestsByTop(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error) {
	var sb strings.Builder
	args := []any{}
	argIdx := 1

	sb.WriteString(`
SELECT fr.id, fr.author_id, fr.title, fr.description, fr.vote_count, fr.created_at, fr.updated_at,
       EXISTS(SELECT 1 FROM votes v WHERE v.request_id = fr.id AND v.user_id = $`)
	sb.WriteString(fmt.Sprintf("%d", argIdx))
	sb.WriteString(`) AS viewer_has_voted FROM feature_requests fr`)
	args = append(args, params.ViewerID)
	argIdx++

	if params.Cursor != nil && *params.Cursor != "" {
		cur, err := decodeCursorRaw(*params.Cursor)
		if err == nil {
			sb.WriteString(fmt.Sprintf(` WHERE (fr.vote_count, fr.created_at, fr.id) < ($%d, $%d, $%d)`,
				argIdx, argIdx+1, argIdx+2))
			args = append(args, cur.voteCount, cur.createdAt, cur.id)
			argIdx += 3
		}
	}

	sb.WriteString(fmt.Sprintf(` ORDER BY fr.vote_count DESC, fr.created_at DESC, fr.id DESC LIMIT $%d`, argIdx))
	args = append(args, params.Limit)

	rows, err := r.pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list by top: %w", err)
	}
	defer rows.Close()
	return collectRequests(rows)
}

func (r *RequestRepo) ListRequestsByTrending(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error) {
	return r.ListRequestsByTop(ctx, params)
}

type rawCursor struct {
	id        string
	createdAt time.Time
	voteCount int32
}

func decodeCursorRaw(encoded string) (*rawCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	parts := strings.SplitN(string(b), "|", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid cursor format: expected 3 parts, got %d", len(parts))
	}
	var nanos int64
	if _, err := fmt.Sscanf(parts[1], "%d", &nanos); err != nil {
		return nil, fmt.Errorf("invalid cursor timestamp: %w", err)
	}
	var voteCount int32
	if _, err := fmt.Sscanf(parts[2], "%d", &voteCount); err != nil {
		return nil, fmt.Errorf("invalid cursor vote_count: %w", err)
	}
	return &rawCursor{id: parts[0], createdAt: time.Unix(0, nanos).UTC(), voteCount: voteCount}, nil
}

func scanRequest(row pgx.Row) (*service.FeatureRequest, error) {
	var fr service.FeatureRequest
	var id, authorID pgtype.UUID
	var createdAt, updatedAt pgtype.Timestamptz
	err := row.Scan(&id, &authorID, &fr.Title, &fr.Description, &fr.VoteCount, &createdAt, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("scan request: %w", err)
	}
	fr.ID = id.String()
	fr.AuthorID = authorID.String()
	fr.CreatedAt = createdAt.Time
	fr.UpdatedAt = updatedAt.Time
	return &fr, nil
}

func scanRequestWithVote(row pgx.Row) (*service.FeatureRequest, error) {
	var fr service.FeatureRequest
	var id, authorID pgtype.UUID
	var createdAt, updatedAt pgtype.Timestamptz
	err := row.Scan(&id, &authorID, &fr.Title, &fr.Description, &fr.VoteCount, &createdAt, &updatedAt, &fr.ViewerHasVoted)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("scan request: %w", err)
	}
	fr.ID = id.String()
	fr.AuthorID = authorID.String()
	fr.CreatedAt = createdAt.Time
	fr.UpdatedAt = updatedAt.Time
	return &fr, nil
}

func collectRequests(rows pgx.Rows) ([]service.FeatureRequest, error) {
	var result []service.FeatureRequest
	for rows.Next() {
		var fr service.FeatureRequest
		var id, authorID pgtype.UUID
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&id, &authorID, &fr.Title, &fr.Description, &fr.VoteCount, &createdAt, &updatedAt, &fr.ViewerHasVoted); err != nil {
			return nil, fmt.Errorf("scan request row: %w", err)
		}
		fr.ID = id.String()
		fr.AuthorID = authorID.String()
		fr.CreatedAt = createdAt.Time
		fr.UpdatedAt = updatedAt.Time
		result = append(result, fr)
	}
	return result, rows.Err()
}
