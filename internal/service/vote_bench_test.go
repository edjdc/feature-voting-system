package service_test

import (
	"context"
	"testing"

	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

func BenchmarkVoteUpvote(b *testing.B) {
	repo := &mockVoteRepo{
		getAuthorFn: func(_ context.Context, _ string) (string, error) {
			return "alice-id", nil
		},
		insertVoteFn: func(_ context.Context, _, _ string) (bool, error) {
			return true, nil
		},
		getCountFn: func(_ context.Context, _ string) (int32, error) {
			return 1, nil
		},
	}
	svc := service.NewVoteService(repo, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Upvote(ctx, "bob-id", "request-1")
	}
}
