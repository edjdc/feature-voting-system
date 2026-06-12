package service

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrSelfVote      = errors.New("cannot vote on your own request")
	ErrAlreadyVoted  = errors.New("already voted")
	ErrValidation    = errors.New("validation failed")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrConflict      = errors.New("conflict")
)
