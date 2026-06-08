package users

import (
	"chess-monolith/pkg/jwtutil"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Register(username string, email string, password string) error
	Login(email string, password string) (string, error)
	VerifyEmail(email string, code string) error
	ResendVerificationCode(email string) error
	GetCurrentUser(token string) (*UserProfile, error)
	GetCurrentUserRatings(token string) ([]UserRatingDTO, error)
	GetLeaderboard(scope RatingScope, limit int) (*LeaderboardDTO, error)
}

type UserProfile struct {
	ID            string          `json:"id"`
	Username      string          `json:"username"`
	Email         string          `json:"email"`
	Rating        int             `json:"rating"`
	EmailVerified bool            `json:"email_verified"`
	Ratings       []UserRatingDTO `json:"ratings"`
}

type service struct {
	repo        Repository
	jwtSecret   string
	emailSender EmailSender
	now         func() time.Time
}

func NewService(repo Repository, jwtSecret string) Service {
	return NewServiceWithEmailSender(repo, jwtSecret, NewLogEmailSender())
}

func NewServiceWithEmailSender(repo Repository, jwtSecret string, emailSender EmailSender) Service {
	if emailSender == nil {
		emailSender = NewLogEmailSender()
	}

	return &service{
		repo:        repo,
		jwtSecret:   jwtSecret,
		emailSender: emailSender,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (s *service) Register(username string, email string, password string) error {
	username = strings.TrimSpace(username)
	email = normalizeEmail(email)

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

	code, codeHash, expiresAt, err := s.newVerificationCode()
	if err != nil {
		return err
	}

	user := &User{
		Username:                   username,
		Email:                      email,
		PasswordHash:               string(hashedBytes),
		EmailVerified:              false,
		EmailVerificationCodeHash:  codeHash,
		EmailVerificationExpiresAt: &expiresAt,
	}

	// Перед CreateUser будет вызван BeforeCreate, который сгенерирует UUID для пользователя
	if err := s.repo.CreateUser(user); err != nil {
		return err
	}

	if err := s.emailSender.SendVerificationCode(email, code); err != nil {
		return err
	}

	return nil
}

func (s *service) Login(email string, password string) (string, error) {
	email = normalizeEmail(email)

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

	if !user.EmailVerified {
		if err := s.refreshVerificationCode(user); err != nil {
			return "", err
		}
		return "", ErrEmailNotVerified
	}

	token, err := jwtutil.GenerateToken(user.ID.String(), s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT: %w", err)
	}

	return token, nil
}

func (s *service) VerifyEmail(email string, code string) error {
	email = normalizeEmail(email)
	code = strings.TrimSpace(code)

	if email == "" || code == "" {
		return ErrInvalidCode
	}

	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrInvalidCode
		}

		return err
	}

	if user.EmailVerified {
		return nil
	}

	if user.EmailVerificationExpiresAt == nil || strings.TrimSpace(user.EmailVerificationCodeHash) == "" {
		return ErrInvalidCode
	}
	if s.now().After(user.EmailVerificationExpiresAt.UTC()) {
		return ErrCodeExpired
	}
	if !verificationCodeMatches(user.EmailVerificationCodeHash, code) {
		return ErrInvalidCode
	}

	return s.repo.MarkEmailVerified(user.ID, s.now())
}

func (s *service) ResendVerificationCode(email string) error {
	email = normalizeEmail(email)
	if email == "" {
		return nil
	}

	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil
		}

		return err
	}

	if user.EmailVerified {
		return nil
	}

	return s.refreshVerificationCode(user)
}

func (s *service) refreshVerificationCode(user *User) error {
	if user == nil || user.EmailVerified {
		return nil
	}

	email := normalizeEmail(user.Email)
	if email == "" {
		return ErrEmailDelivery
	}

	code, codeHash, expiresAt, err := s.newVerificationCode()
	if err != nil {
		return err
	}

	if err := s.repo.UpdateEmailVerification(user.ID, codeHash, expiresAt); err != nil {
		return err
	}

	if err := s.emailSender.SendVerificationCode(email, code); err != nil {
		return err
	}

	return nil
}

func (s *service) GetCurrentUser(token string) (*UserProfile, error) {
	parsedUserID, err := s.userIDFromToken(token)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(parsedUserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrUnauthorized
		}

		return nil, err
	}

	ratings, err := s.ratingsForUser(parsedUserID)
	if err != nil {
		return nil, err
	}

	return &UserProfile{
		ID:            user.ID.String(),
		Username:      user.Username,
		Email:         user.Email,
		Rating:        profileSummaryRating(user.Rating, ratings),
		EmailVerified: user.EmailVerified,
		Ratings:       ratings,
	}, nil
}

func (s *service) GetCurrentUserRatings(token string) ([]UserRatingDTO, error) {
	parsedUserID, err := s.userIDFromToken(token)
	if err != nil {
		return nil, err
	}

	return s.ratingsForUser(parsedUserID)
}

func (s *service) GetLeaderboard(scope RatingScope, limit int) (*LeaderboardDTO, error) {
	scope = normalizeRatingScope(scope)
	rows, err := s.repo.ListLeaderboard(scope, limit)
	if err != nil {
		return nil, err
	}

	players := make([]LeaderboardEntryDTO, 0, len(rows))
	for index, row := range rows {
		players = append(players, LeaderboardEntryDTO{
			Rank:        index + 1,
			UserID:      row.UserID.String(),
			Username:    row.Username,
			Rating:      row.Rating,
			GamesPlayed: row.GamesPlayed,
		})
	}

	return &LeaderboardDTO{
		Scope:   ratingScopeDTO(scope),
		Players: players,
	}, nil
}

func (s *service) userIDFromToken(token string) (uuid.UUID, error) {
	userID, err := jwtutil.ParseToken(token, s.jwtSecret)
	if err != nil {
		return uuid.Nil, ErrUnauthorized
	}

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil, ErrUnauthorized
	}

	return parsedUserID, nil
}

func (s *service) ratingsForUser(userID uuid.UUID) ([]UserRatingDTO, error) {
	existingRatings, err := s.repo.ListRatingsForUser(userID)
	if err != nil {
		return nil, err
	}

	ratingsByScope := make(map[ratingScopeMapKey]UserRatingDTO, len(existingRatings))
	for _, rating := range existingRatings {
		ratingsByScope[ratingScopeKey(RatingScope{
			Mode:        rating.Mode,
			BoardSize:   rating.BoardSize,
			TimeLimitMs: rating.TimeLimitMs,
		})] = userRatingDTO(rating)
	}

	ratings := make([]UserRatingDTO, 0, len(supportedRatingScopes())+len(existingRatings))
	includedScopes := make(map[ratingScopeMapKey]struct{}, len(existingRatings))
	for _, scope := range supportedRatingScopes() {
		key := ratingScopeKey(scope)
		if rating, ok := ratingsByScope[key]; ok {
			ratings = append(ratings, rating)
			includedScopes[key] = struct{}{}
			continue
		}

		ratings = append(ratings, defaultUserRatingDTO(scope))
		includedScopes[key] = struct{}{}
	}

	for _, rating := range existingRatings {
		scope := RatingScope{
			Mode:        rating.Mode,
			BoardSize:   rating.BoardSize,
			TimeLimitMs: rating.TimeLimitMs,
		}
		key := ratingScopeKey(scope)
		if _, ok := includedScopes[key]; ok {
			continue
		}
		ratings = append(ratings, userRatingDTO(rating))
		includedScopes[key] = struct{}{}
	}

	return ratings, nil
}

func (s *service) newVerificationCode() (string, string, time.Time, error) {
	code, err := generateVerificationCode()
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to generate verification code: %w", err)
	}

	codeHash, err := hashVerificationCode(code)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to hash verification code: %w", err)
	}

	return code, codeHash, s.now().Add(emailVerificationTTL), nil
}

func profileSummaryRating(legacyRating int, ratings []UserRatingDTO) int {
	summaryRating := legacyRating
	hasScopedGames := false

	for _, rating := range ratings {
		if rating.GamesPlayed <= 0 {
			continue
		}
		if !hasScopedGames || rating.Rating > summaryRating {
			summaryRating = rating.Rating
		}
		hasScopedGames = true
	}

	return summaryRating
}
