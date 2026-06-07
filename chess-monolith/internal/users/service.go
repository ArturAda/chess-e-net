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
	GetCurrentUserRatings(token string) ([]UserRatingDTO, error)
	GetLeaderboard(scope RatingScope, limit int) (*LeaderboardDTO, error)
}

type UserProfile struct {
	ID       string          `json:"id"`
	Username string          `json:"username"`
	Email    string          `json:"email"`
	Rating   int             `json:"rating"`
	Ratings  []UserRatingDTO `json:"ratings"`
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
		ID:       user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		Rating:   profileSummaryRating(user.Rating, ratings),
		Ratings:  ratings,
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

	ratings := make([]UserRatingDTO, 0, len(supportedRatingScopes()))
	for _, scope := range supportedRatingScopes() {
		key := ratingScopeKey(scope)
		if rating, ok := ratingsByScope[key]; ok {
			ratings = append(ratings, rating)
			continue
		}

		ratings = append(ratings, defaultUserRatingDTO(scope))
	}

	return ratings, nil
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
