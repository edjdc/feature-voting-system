package service

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"

	"crypto/rand"
	"encoding/base64"
	"strings"
)

const (
	argon2Memory      = 64 * 1024
	argon2Iterations  = 3
	argon2Parallelism = 2
	argon2SaltLen     = 16
	argon2KeyLen      = 32
)

type TokenClaims struct {
	UserID string
}

type AuthService struct {
	accessSecret  string
	refreshSecret string
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewAuthService(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *AuthService {
	return &AuthService{
		accessSecret:  accessSecret,
		refreshSecret: refreshSecret,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

func (s *AuthService) HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLen)

	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

func (s *AuthService) VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	hash := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, uint32(len(expectedHash)))

	if len(hash) != len(expectedHash) {
		return false
	}
	diff := byte(0)
	for i := range hash {
		diff |= hash[i] ^ expectedHash[i]
	}
	return diff == 0
}

func (s *AuthService) IssueAccessToken(userID string) (string, error) {
	return s.issueToken(userID, s.accessSecret, s.accessTTL)
}

func (s *AuthService) IssueRefreshToken(userID string) (string, error) {
	return s.issueToken(userID, s.refreshSecret, s.refreshTTL)
}

func (s *AuthService) VerifyAccessToken(tokenStr string) (*TokenClaims, error) {
	return s.verifyToken(tokenStr, s.accessSecret)
}

func (s *AuthService) VerifyRefreshToken(tokenStr string) (*TokenClaims, error) {
	return s.verifyToken(tokenStr, s.refreshSecret)
}

type jwtClaims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

func (s *AuthService) issueToken(userID, secret string, ttl time.Duration) (string, error) {
	claims := jwtClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func (s *AuthService) verifyToken(tokenStr, secret string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnauthorized, err)
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, ErrUnauthorized
	}

	return &TokenClaims{UserID: claims.UserID}, nil
}
