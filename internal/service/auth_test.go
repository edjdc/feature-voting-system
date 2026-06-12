package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

func TestHashPassword(t *testing.T) {
	svc := service.NewAuthService("access-secret", "refresh-secret", time.Minute, time.Hour)

	hash, err := svc.HashPassword("correct horse battery")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "correct horse battery", hash)

	// Same password hashes differently (salt)
	hash2, err := svc.HashPassword("correct horse battery")
	require.NoError(t, err)
	assert.NotEqual(t, hash, hash2)
}

func TestVerifyPassword(t *testing.T) {
	svc := service.NewAuthService("access-secret", "refresh-secret", time.Minute, time.Hour)

	hash, err := svc.HashPassword("my-password")
	require.NoError(t, err)

	assert.True(t, svc.VerifyPassword(hash, "my-password"))
	assert.False(t, svc.VerifyPassword(hash, "wrong-password"))
}

func TestIssueAndVerifyAccessToken(t *testing.T) {
	svc := service.NewAuthService("access-secret", "refresh-secret", time.Minute, time.Hour)

	userID := "00000000-0000-0000-0000-000000000001"
	token, err := svc.IssueAccessToken(userID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := svc.VerifyAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
}

func TestVerifyAccessToken_InvalidSecret(t *testing.T) {
	svc1 := service.NewAuthService("secret-1", "refresh-secret", time.Minute, time.Hour)
	svc2 := service.NewAuthService("secret-2", "refresh-secret", time.Minute, time.Hour)

	token, err := svc1.IssueAccessToken("user-id")
	require.NoError(t, err)

	_, err = svc2.VerifyAccessToken(token)
	assert.Error(t, err)
}

func TestIssueAndVerifyRefreshToken(t *testing.T) {
	svc := service.NewAuthService("access-secret", "refresh-secret", time.Minute, time.Hour)

	userID := "00000000-0000-0000-0000-000000000001"
	token, err := svc.IssueRefreshToken(userID)
	require.NoError(t, err)

	claims, err := svc.VerifyRefreshToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
}
