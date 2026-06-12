-- name: InsertVote :execresult
-- INSERT with ON CONFLICT DO NOTHING for idempotent upvote
INSERT INTO votes (user_id, request_id) VALUES ($1, $2)
ON CONFLICT (user_id, request_id) DO NOTHING;

-- name: DeleteVote :execresult
DELETE FROM votes WHERE user_id = $1 AND request_id = $2;

-- name: IncrementVoteCount :exec
UPDATE feature_requests
SET vote_count = vote_count + 1, updated_at = now()
WHERE id = $1;

-- name: DecrementVoteCount :exec
UPDATE feature_requests
SET vote_count = GREATEST(vote_count - 1, 0), updated_at = now()
WHERE id = $1;

-- name: GetVoteCount :one
SELECT vote_count FROM feature_requests WHERE id = $1;

-- name: GetRequestAuthorID :one
SELECT author_id FROM feature_requests WHERE id = $1;
