package service

import (
	"context"
	"fmt"
	"log/slog"
)

type VoteResult struct {
	RequestID string `json:"request_id"`
	VoteCount int32  `json:"vote_count"`
	UserVoted bool   `json:"viewer_has_voted"`
}

// VoteCounter is the evolution seam for Option B (event-driven counts).
type VoteCounter interface {
	IncrementVote(ctx context.Context, requestID string) error
	DecrementVote(ctx context.Context, requestID string) error
}

type VoteRepo interface {
	InsertVote(ctx context.Context, userID, requestID string) (bool, error) // returns inserted=true if new
	DeleteVote(ctx context.Context, userID, requestID string) (bool, error) // returns deleted=true if existed
	GetVoteCount(ctx context.Context, requestID string) (int32, error)
	GetRequestAuthorID(ctx context.Context, requestID string) (string, error)
}

type VoteService struct {
	repo VoteRepo
	log  *slog.Logger
}

func NewVoteService(repo VoteRepo, log *slog.Logger) *VoteService {
	return &VoteService{repo: repo, log: log}
}

func (s *VoteService) Upvote(ctx context.Context, userID, requestID string) (*VoteResult, error) {
	authorID, err := s.repo.GetRequestAuthorID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("%w: feature request not found", ErrNotFound)
	}

	if authorID == userID {
		return nil, ErrSelfVote
	}

	_, err = s.repo.InsertVote(ctx, userID, requestID)
	if err != nil {
		return nil, fmt.Errorf("insert vote: %w", err)
	}

	count, err := s.repo.GetVoteCount(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("get vote count: %w", err)
	}

	return &VoteResult{
		RequestID: requestID,
		VoteCount: count,
		UserVoted: true,
	}, nil
}

func (s *VoteService) RemoveVote(ctx context.Context, userID, requestID string) (*VoteResult, error) {
	_, err := s.repo.GetRequestAuthorID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("%w: feature request not found", ErrNotFound)
	}

	_, err = s.repo.DeleteVote(ctx, userID, requestID)
	if err != nil {
		return nil, fmt.Errorf("delete vote: %w", err)
	}

	count, err := s.repo.GetVoteCount(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("get vote count: %w", err)
	}

	return &VoteResult{
		RequestID: requestID,
		VoteCount: count,
		UserVoted: false,
	}, nil
}
