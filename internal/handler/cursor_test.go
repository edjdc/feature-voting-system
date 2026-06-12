package handler_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/handler"
)

func TestCursorRoundTrip(t *testing.T) {
	id := "550e8400-e29b-41d4-a716-446655440000"
	createdAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	voteCount := int32(42)

	encoded := handler.EncodeCursor(id, createdAt, voteCount)
	assert.NotEmpty(t, encoded)

	decoded, err := handler.DecodeCursor(encoded)
	require.NoError(t, err)
	assert.Equal(t, id, decoded.ID)
	assert.Equal(t, createdAt.UnixNano(), decoded.CreatedAt.UnixNano())
	assert.Equal(t, voteCount, decoded.VoteCount)
}

func TestCursorTamperDetection(t *testing.T) {
	_, err := handler.DecodeCursor("not-valid-base64!!!")
	assert.Error(t, err)

	_, err = handler.DecodeCursor("aW52YWxpZA")  // "invalid" in base64 — no pipe separators
	assert.Error(t, err)
}

func TestCursorEmptyID(t *testing.T) {
	encoded := handler.EncodeCursor("", time.Now(), 0)
	decoded, err := handler.DecodeCursor(encoded)
	require.NoError(t, err)
	assert.Equal(t, "", decoded.ID)
}
