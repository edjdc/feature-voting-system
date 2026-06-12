package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

type UserRecord struct {
	ID           string
	Email        string
	PasswordHash string
}

type UserRepo interface {
	CreateUser(ctx context.Context, email, passwordHash string) (UserRecord, error)
	GetUserByEmail(ctx context.Context, email string) (UserRecord, error)
}

type UserService struct {
	repo UserRepo
	auth *AuthService
	log  *slog.Logger
}

func NewUserService(repo UserRepo, auth *AuthService, log *slog.Logger) *UserService {
	return &UserService{repo: repo, auth: auth, log: log}
}

func (s *UserService) Register(ctx context.Context, email, password string) (*UserRecord, error) {
	hash, err := s.auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, email, hash)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return &user, nil
}

func (s *UserService) Login(ctx context.Context, email, password string) (*UserRecord, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if user.PasswordHash == "" || !s.auth.VerifyPassword(user.PasswordHash, password) {
		return nil, ErrUnauthorized
	}

	return &user, nil
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsAny(msg, "duplicate key", "unique constraint", "23505")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
