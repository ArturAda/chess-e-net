package users

import "errors"

var (
	ErrDatabase           = errors.New("database operation failed")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrEmailNotVerified   = errors.New("email is not verified")
	ErrInvalidCode        = errors.New("invalid verification code")
	ErrCodeExpired        = errors.New("verification code expired")
	ErrEmailDelivery      = errors.New("failed to send verification email")
	ErrCaptchaRequired    = errors.New("captcha token is required")
	ErrCaptchaInvalid     = errors.New("captcha verification failed")
	ErrCaptchaUnavailable = errors.New("captcha verification is unavailable")
)
