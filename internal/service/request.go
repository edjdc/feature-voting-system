package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	maxTitleLen       = 100
	maxDescriptionLen = 5000
)

type FeatureRequest struct {
	ID          string    `json:"id"`
	AuthorID    string    `json:"author_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	VoteCount   int32     `json:"vote_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	ViewerHasVoted bool `json:"viewer_has_voted"`
}

type RequestPage struct {
	Items      []FeatureRequest `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

type ListParams struct {
	Sort       string
	Cursor     *string
	Limit      int
	ViewerID   string
}

type RequestRepo interface {
	InsertFeatureRequest(ctx context.Context, authorID, title, description string) (*FeatureRequest, error)
	ListRequestsByNew(ctx context.Context, params ListParams) ([]FeatureRequest, error)
	ListRequestsByTop(ctx context.Context, params ListParams) ([]FeatureRequest, error)
	GetFeatureRequest(ctx context.Context, id, viewerID string) (*FeatureRequest, error)
}

type RequestService struct {
	repo RequestRepo
	log  *slog.Logger
}

func NewRequestService(repo RequestRepo, log *slog.Logger) *RequestService {
	return &RequestService{repo: repo, log: log}
}

func (s *RequestService) Submit(ctx context.Context, authorID, title, description string) (*FeatureRequest, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)

	if title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if len(title) > maxTitleLen {
		return nil, fmt.Errorf("%w: title must be %d characters or fewer", ErrValidation, maxTitleLen)
	}
	if description == "" {
		return nil, fmt.Errorf("%w: description is required", ErrValidation)
	}
	if len(description) > maxDescriptionLen {
		return nil, fmt.Errorf("%w: description must be %d characters or fewer", ErrValidation, maxDescriptionLen)
	}

	req, err := s.repo.InsertFeatureRequest(ctx, authorID, title, description)
	if err != nil {
		return nil, fmt.Errorf("insert feature request: %w", err)
	}
	return req, nil
}

func (s *RequestService) List(ctx context.Context, params ListParams) (*RequestPage, error) {
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}

	// Fetch one extra to determine if there's a next page
	fetchLimit := params.Limit + 1

	fetchParams := ListParams{
		Sort:     params.Sort,
		Cursor:   params.Cursor,
		Limit:    fetchLimit,
		ViewerID: params.ViewerID,
	}

	var items []FeatureRequest
	var err error

	switch params.Sort {
	case "top":
		items, err = s.repo.ListRequestsByTop(ctx, fetchParams)
	case "trending":
		// Trending uses the same underlying list but is sorted differently by the ranking service
		items, err = s.repo.ListRequestsByTop(ctx, fetchParams)
	default: // "new"
		items, err = s.repo.ListRequestsByNew(ctx, fetchParams)
	}
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}

	var nextCursor *string
	if len(items) > params.Limit {
		items = items[:params.Limit]
		last := items[len(items)-1]
		cursor := encodeCursor(last)
		nextCursor = &cursor
	}

	return &RequestPage{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}

func (s *RequestService) GetByID(ctx context.Context, id, viewerID string) (*FeatureRequest, error) {
	req, err := s.repo.GetFeatureRequest(ctx, id, viewerID)
	if err != nil {
		return nil, fmt.Errorf("get feature request: %w", err)
	}
	return req, nil
}

func encodeCursor(req FeatureRequest) string {
	raw := fmt.Sprintf("%s|%d|%d", req.ID, req.CreatedAt.UnixNano(), req.VoteCount)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}
