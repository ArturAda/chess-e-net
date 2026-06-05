package users

import (
	"chess-monolith/pkg/jwtutil"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Register(username string, email string, password string) error
	Login(email string, password string) (string, error)
	GetCurrentUser(token string) (*UserProfile, error)
}

type UserProfile struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Rating   int    `json:"rating"`
}

type service struct {
	repo      Repository
	jwtSecret string
}

func NewService(repo Repository, jwtSecret string) Service {
	return &service{repo: repo, jwtSecret: jwtSecret}
}

func (s *service) Register(username string, email string, password string) error {
	_, err := s.repo.GetUserByEmail(email)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			return err
		}
	} else {
		return ErrUserExists
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := &User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hashedBytes),
	}

	// Перед CreateUser будет вызван BeforeCreate, который сгенерирует UUID для пользователя
	return s.repo.CreateUser(user)
}

func (s *service) Login(email string, password string) (string, error) {
	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", ErrInvalidCredentials
		}

		return "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", ErrInvalidCredentials
	}

	token, err := jwtutil.GenerateToken(user.ID.String(), s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT: %w", err)
	}

	return token, nil
}

func (s *service) GetCurrentUser(token string) (*UserProfile, error) {
	userID, err := jwtutil.ParseToken(token, s.jwtSecret)
	if err != nil {
		return nil, ErrUnauthorized
	}

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrUnauthorized
	}

	user, err := s.repo.GetUserByID(parsedUserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrUnauthorized
		}

		return nil, err
	}

	return &UserProfile{
		ID:       user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		Rating:   user.Rating,
	}, nil
}
