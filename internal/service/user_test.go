package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type stubUserRepo2 struct {
	createFn func(ctx context.Context, email, passwordHash string) (service.UserRecord, error)
	getFn    func(ctx context.Context, email string) (service.UserRecord, error)
}

func (s *stubUserRepo2) CreateUser(ctx context.Context, email, passwordHash string) (service.UserRecord, error) {
	return s.createFn(ctx, email, passwordHash)
}
func (s *stubUserRepo2) GetUserByEmail(ctx context.Context, email string) (service.UserRecord, error) {
	return s.getFn(ctx, email)
}

func TestUserService_Register_Success(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	repo := &stubUserRepo2{
		createFn: func(_ context.Context, email, _ string) (service.UserRecord, error) {
			return service.UserRecord{ID: "user-1", Email: email}, nil
		},
	}
	svc := service.NewUserService(repo, authSvc, log)

	user, err := svc.Register(context.Background(), "alice@example.com", "p4ssw0rd!")
	require.NoError(t, err)
	assert.Equal(t, "user-1", user.ID)
	assert.Equal(t, "alice@example.com", user.Email)
}

func TestUserService_Register_DuplicateEmail(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	repo := &stubUserRepo2{
		createFn: func(_ context.Context, _, _ string) (service.UserRecord, error) {
			return service.UserRecord{}, errors.New("duplicate key value violates unique constraint")
		},
	}
	svc := service.NewUserService(repo, authSvc, log)

	_, err := svc.Register(context.Background(), "alice@example.com", "pass")
	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrConflict)
}

func TestUserService_Login_Success(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	hash, _ := authSvc.HashPassword("correct-pass")
	repo := &stubUserRepo2{
		getFn: func(_ context.Context, email string) (service.UserRecord, error) {
			return service.UserRecord{ID: "user-1", Email: email, PasswordHash: hash}, nil
		},
	}
	svc := service.NewUserService(repo, authSvc, log)

	user, err := svc.Login(context.Background(), "alice@example.com", "correct-pass")
	require.NoError(t, err)
	assert.Equal(t, "user-1", user.ID)
}

func TestUserService_Login_WrongPassword(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	hash, _ := authSvc.HashPassword("correct")
	repo := &stubUserRepo2{
		getFn: func(_ context.Context, email string) (service.UserRecord, error) {
			return service.UserRecord{ID: "user-1", Email: email, PasswordHash: hash}, nil
		},
	}
	svc := service.NewUserService(repo, authSvc, log)

	_, err := svc.Login(context.Background(), "alice@example.com", "wrong")
	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrUnauthorized)
}

func TestUserService_Login_UserNotFound(t *testing.T) {
	log := observability.NewLogger()
	authSvc := service.NewAuthService("secret", "refresh", time.Minute, time.Hour)
	repo := &stubUserRepo2{
		getFn: func(_ context.Context, _ string) (service.UserRecord, error) {
			return service.UserRecord{}, pgx.ErrNoRows
		},
	}
	svc := service.NewUserService(repo, authSvc, log)

	_, err := svc.Login(context.Background(), "nobody@example.com", "pass")
	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrUnauthorized)
}
