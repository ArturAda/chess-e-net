package users

import "errors"

var (
	ErrDatabase           = errors.New("database operation failed")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
)
