package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type mockVoteRepo struct {
	getAuthorFn  func(ctx context.Context, requestID string) (string, error)
	insertVoteFn func(ctx context.Context, userID, requestID string) (bool, error)
	deleteVoteFn func(ctx context.Context, userID, requestID string) (bool, error)
	getCountFn   func(ctx context.Context, requestID string) (int32, error)
}

func (m *mockVoteRepo) GetRequestAuthorID(ctx context.Context, requestID string) (string, error) {
	return m.getAuthorFn(ctx, requestID)
}
func (m *mockVoteRepo) InsertVote(ctx context.Context, userID, requestID string) (bool, error) {
	return m.insertVoteFn(ctx, userID, requestID)
}
func (m *mockVoteRepo) DeleteVote(ctx context.Context, userID, requestID string) (bool, error) {
	return m.deleteVoteFn(ctx, userID, requestID)
}
func (m *mockVoteRepo) GetVoteCount(ctx context.Context, requestID string) (int32, error) {
	return m.getCountFn(ctx, requestID)
}

func TestVoteUpvote_SelfVoteRejected(t *testing.T) {
	repo := &mockVoteRepo{
		getAuthorFn: func(_ context.Context, _ string) (string, error) {
			return "alice-id", nil
		},
	}
	svc := service.NewVoteService(repo, nil)

	_, err := svc.Upvote(context.Background(), "alice-id", "request-1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrSelfVote))
}

func TestVoteUpvote_Idempotent(t *testing.T) {
	voteCount := int32(1)
	repo := &mockVoteRepo{
		getAuthorFn: func(_ context.Context, _ string) (string, error) {
			return "alice-id", nil
		},
		insertVoteFn: func(_ context.Context, _, _ string) (bool, error) {
			return false, nil // already voted, no-op
		},
		getCountFn: func(_ context.Context, _ string) (int32, error) {
			return voteCount, nil
		},
	}
	svc := service.NewVoteService(repo, nil)

	result, err := svc.Upvote(context.Background(), "bob-id", "request-1")
	require.NoError(t, err)
	assert.Equal(t, int32(1), result.VoteCount)
	assert.True(t, result.UserVoted)
}

func TestVoteUpvote_NotFound(t *testing.T) {
	repo := &mockVoteRepo{
		getAuthorFn: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("not found")
		},
	}
	svc := service.NewVoteService(repo, nil)

	_, err := svc.Upvote(context.Background(), "bob-id", "nonexistent-request")
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrNotFound))
}

func TestVoteRemove_Success(t *testing.T) {
	repo := &mockVoteRepo{
		getAuthorFn: func(_ context.Context, _ string) (string, error) {
			return "alice-id", nil
		},
		deleteVoteFn: func(_ context.Context, _, _ string) (bool, error) {
			return true, nil
		},
		getCountFn: func(_ context.Context, _ string) (int32, error) {
			return 0, nil
		},
	}
	svc := service.NewVoteService(repo, nil)

	result, err := svc.RemoveVote(context.Background(), "bob-id", "request-1")
	require.NoError(t, err)
	assert.Equal(t, int32(0), result.VoteCount)
	assert.False(t, result.UserVoted)
}
