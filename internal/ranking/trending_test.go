package ranking_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/edivilsondalacosta/feature-voting-system/internal/ranking"
)

func TestTrendingScore_Formula(t *testing.T) {
	// score = vote_count / (age_hours + 2)^1.5
	tests := []struct {
		name      string
		voteCount int32
		ageHours  float64
		expected  float64
	}{
		{
			name:      "zero_age",
			voteCount: 100,
			ageHours:  0,
			expected:  100.0 / math.Pow(2, 1.5),
		},
		{
			name:      "one_hour_old",
			voteCount: 100,
			ageHours:  1,
			expected:  100.0 / math.Pow(3, 1.5),
		},
		{
			name:      "zero_votes",
			voteCount: 0,
			ageHours:  10,
			expected:  0,
		},
		{
			name:      "older_lower_score",
			voteCount: 50,
			ageHours:  48,
			expected:  50.0 / math.Pow(50, 1.5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdAt := time.Now().Add(-time.Duration(tt.ageHours * float64(time.Hour)))
			score := ranking.TrendingScore(tt.voteCount, createdAt)
			assert.InDelta(t, tt.expected, score, 0.1, "score mismatch for %s", tt.name)
		})
	}
}

func TestTrendingScore_RecentBeatsOld(t *testing.T) {
	sameVotes := int32(100)

	oldCreatedAt := time.Now().Add(-48 * time.Hour)
	recentCreatedAt := time.Now().Add(-1 * time.Hour)

	oldScore := ranking.TrendingScore(sameVotes, oldCreatedAt)
	recentScore := ranking.TrendingScore(sameVotes, recentCreatedAt)

	assert.Greater(t, recentScore, oldScore, "recent request should score higher than old request with same votes")
}

func TestTrendingScore_TiebreakDeterministic(t *testing.T) {
	// TrendingScore uses time.Since which advances between calls;
	// verify the two calls produce the same score within float64 precision.
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	voteCount := int32(50)

	score1 := ranking.TrendingScore(voteCount, createdAt)
	score2 := ranking.TrendingScore(voteCount, createdAt)

	// Same createdAt → nearly equal; delta accounts for sub-millisecond time difference
	assert.InDelta(t, score1, score2, 1e-6, "same inputs must produce nearly-equal scores")
}
