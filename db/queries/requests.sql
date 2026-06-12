-- name: InsertFeatureRequest :one
INSERT INTO feature_requests (author_id, title, description)
VALUES ($1, $2, $3)
RETURNING id, author_id, title, description, vote_count, created_at, updated_at;

-- name: ListRequestsByNew :many
SELECT fr.id, fr.author_id, fr.title, fr.description, fr.vote_count, fr.created_at, fr.updated_at,
       EXISTS(SELECT 1 FROM votes v WHERE v.request_id = fr.id AND v.user_id = $1) AS viewer_has_voted
FROM feature_requests fr
ORDER BY fr.created_at DESC, fr.id DESC
LIMIT $2;

-- name: ListRequestsByTop :many
SELECT fr.id, fr.author_id, fr.title, fr.description, fr.vote_count, fr.created_at, fr.updated_at,
       EXISTS(SELECT 1 FROM votes v WHERE v.request_id = fr.id AND v.user_id = $1) AS viewer_has_voted
FROM feature_requests fr
ORDER BY fr.vote_count DESC, fr.created_at DESC, fr.id DESC
LIMIT $2;

-- name: GetFeatureRequest :one
SELECT fr.id, fr.author_id, fr.title, fr.description, fr.vote_count, fr.created_at, fr.updated_at,
       EXISTS(SELECT 1 FROM votes v WHERE v.request_id = fr.id AND v.user_id = $2) AS viewer_has_voted
FROM feature_requests fr
WHERE fr.id = $1;
