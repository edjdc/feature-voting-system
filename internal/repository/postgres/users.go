package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

const createUserSQL = `
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING id, email, password_hash, created_at`

func (r *UserRepository) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	row := r.pool.QueryRow(ctx, createUserSQL, email, passwordHash)
	user, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

const getUserByEmailSQL = `
SELECT id, email, password_hash, created_at FROM users WHERE email = $1`

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	row := r.pool.QueryRow(ctx, getUserByEmailSQL, email)
	user, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

const getUserByIDSQL = `
SELECT id, email, password_hash, created_at FROM users WHERE id = $1`

func (r *UserRepository) GetUserByID(ctx context.Context, id pgtype.UUID) (*User, error) {
	row := r.pool.QueryRow(ctx, getUserByIDSQL, id)
	user, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
