package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) CreateUser(ctx context.Context, email, passwordHash string) (service.UserRecord, error) {
	row := r.pool.QueryRow(ctx, createUserSQL, email, passwordHash)
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt); err != nil {
		return service.UserRecord{}, fmt.Errorf("create user: %w", err)
	}
	return service.UserRecord{
		ID:           u.ID.String(),
		Email:        u.Email,
		PasswordHash: derefStr(u.PasswordHash),
	}, nil
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (service.UserRecord, error) {
	row := r.pool.QueryRow(ctx, getUserByEmailSQL, email)
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt); err != nil {
		return service.UserRecord{}, fmt.Errorf("get user by email: %w", err)
	}
	return service.UserRecord{
		ID:           u.ID.String(),
		Email:        u.Email,
		PasswordHash: derefStr(u.PasswordHash),
	}, nil
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
