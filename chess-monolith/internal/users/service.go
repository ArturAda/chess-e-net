package users

import (
	"chess-monolith/pkg/jwtutil"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Register(username string, email string, password string) error
	Login(email string, password string) (string, error)
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
	if err == nil {
		return ErrUserExists
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
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
		return "", ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", ErrInvalidCredentials
	}

	token, err := jwtutil.GenerateToken(user.ID.String(), s.jwtSecret)
	if err != nil {
		return "", err
	}

	return token, nil
}
