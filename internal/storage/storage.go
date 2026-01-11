package storage

import "errors"

var (
	ErrUserExists      = errors.New("user already exists")
	ErrUserNotFound    = errors.New("user not found")
	ErrAppNotFound     = errors.New("app not found")
	ErrUserAppNotFound = errors.New("userApp not found")
	ErrUserAppExists   = errors.New("userApp already exists")
)
