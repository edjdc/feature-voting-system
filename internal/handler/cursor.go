package handler

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// CursorData holds decoded cursor fields.
type CursorData struct {
	ID        string
	CreatedAt time.Time
	VoteCount int32
}

// EncodeCursor produces an opaque cursor string from the last item in a page.
func EncodeCursor(id string, createdAt time.Time, voteCount int32) string {
	raw := fmt.Sprintf("%s|%d|%d", id, createdAt.UnixNano(), voteCount)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses an opaque cursor back into its components.
func DecodeCursor(encoded string) (*CursorData, error) {
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

	return &CursorData{
		ID:        parts[0],
		CreatedAt: time.Unix(0, nanos).UTC(),
		VoteCount: voteCount,
	}, nil
}
