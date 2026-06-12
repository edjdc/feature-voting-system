package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
)

type RankingRepo interface {
	ListRequestsByTop(ctx context.Context, params ListParams) ([]FeatureRequest, error)
	ListRequestsByTrending(ctx context.Context, params ListParams) ([]FeatureRequest, error)
	ListRequestsByNew(ctx context.Context, params ListParams) ([]FeatureRequest, error)
}

type RankingService struct {
	repo RankingRepo
	log  *slog.Logger
}

func NewRankingService(repo RankingRepo, log *slog.Logger) *RankingService {
	return &RankingService{repo: repo, log: log}
}

func (s *RankingService) List(ctx context.Context, params ListParams) (*RequestPage, error) {
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}

	fetchParams := ListParams{
		Sort:     params.Sort,
		Cursor:   params.Cursor,
		Limit:    params.Limit + 1,
		ViewerID: params.ViewerID,
	}

	var items []FeatureRequest
	var err error

	switch params.Sort {
	case "top":
		items, err = s.repo.ListRequestsByTop(ctx, fetchParams)
	case "trending":
		items, err = s.repo.ListRequestsByTrending(ctx, fetchParams)
	default: // "new"
		items, err = s.repo.ListRequestsByNew(ctx, fetchParams)
	}
	if err != nil {
		return nil, fmt.Errorf("list requests (%s): %w", params.Sort, err)
	}

	var nextCursor *string
	if len(items) > params.Limit {
		items = items[:params.Limit]
		last := items[len(items)-1]
		raw := fmt.Sprintf("%s|%d|%d", last.ID, last.CreatedAt.UnixNano(), last.VoteCount)
		cursor := base64.RawURLEncoding.EncodeToString([]byte(raw))
		nextCursor = &cursor
	}

	return &RequestPage{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}
