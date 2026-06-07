package users

import (
	"chess-monolith/pkg/jwtutil"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// MockRepository - имитация репозитория
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateUser(user *User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockRepository) GetUserByEmail(email string) (*User, error) {
	// Забирает Return значения из мок-метода и приводит их к нужному типу
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *MockRepository) GetUserByID(id uuid.UUID) (*User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *MockRepository) UpdateRatings(winnerID, loserID uuid.UUID, winnerRating, loserRating int) error {
	args := m.Called(winnerID, loserID, winnerRating, loserRating)
	return args.Error(0)
}

func (m *MockRepository) GetOrCreateRating(userID uuid.UUID, scope RatingScope) (*UserRating, error) {
	args := m.Called(userID, scope)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UserRating), args.Error(1)
}

func (m *MockRepository) ListRatingsForUser(userID uuid.UUID) ([]UserRating, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]UserRating), args.Error(1)
}

func (m *MockRepository) ListLeaderboard(scope RatingScope, limit int) ([]LeaderboardEntry, error) {
	args := m.Called(scope, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]LeaderboardEntry), args.Error(1)
}

func (m *MockRepository) ApplyRatingResult(user1ID, user2ID uuid.UUID, scope RatingScope, user1Score float64) (int, int, error) {
	args := m.Called(user1ID, user2ID, scope, user1Score)
	return args.Int(0), args.Int(1), args.Error(2)
}

func TestService_Register_UserExists(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewService(mockRepo, "secret")

	// Возвращаем существующего пустую структуру пользователя при запросе по email, что имитирует наличие такого пользователя
	mockRepo.On("GetUserByEmail", "exists@mail.com").Return(&User{}, nil)

	err := service.Register("test", "exists@mail.com", "password")

	assert.Equal(t, ErrUserExists, err)
	mockRepo.AssertExpectations(t)
}

func TestService_Login_InvalidCreds(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewService(mockRepo, "secret")

	// Возвращаем ошибку при попытке получить пользователя по email, что имитирует отсутствие такого пользователя
	mockRepo.On("GetUserByEmail", "wrong@mail.com").Return(nil, ErrUserNotFound)

	token, err := service.Login("wrong@mail.com", "any")

	assert.Empty(t, token)
	assert.Equal(t, ErrInvalidCredentials, err)
}

func TestService_Register_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewService(mockRepo, "secret")

	// 1. Проверяем, что юзера нет (ожидаем ErrUserNotFound)
	mockRepo.On("GetUserByEmail", "new@mail.com").Return(nil, ErrUserNotFound)

	// 2. Ожидаем вызов CreateUser с любым объектом User
	mockRepo.On("CreateUser", mock.AnythingOfType("*users.User")).Return(nil)

	err := service.Register("test", "new@mail.com", "password")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_Register_DatabaseError(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewService(mockRepo, "secret")

	mockRepo.On("GetUserByEmail", "error@mail.com").Return(nil, ErrDatabase)

	err := service.Register("test", "error@mail.com", "password")

	assert.Equal(t, ErrDatabase, err)
	mockRepo.AssertExpectations(t)
}

func TestService_Login_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewService(mockRepo, "secret")

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	validUser := &User{
		ID:           uuid.New(),
		Email:        "test@mail.com",
		PasswordHash: string(hashedPassword),
	}

	mockRepo.On("GetUserByEmail", "test@mail.com").Return(validUser, nil)

	token, err := service.Login("test@mail.com", "password123")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	mockRepo.AssertExpectations(t)
}

func TestService_GetCurrentUser_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	secret := "secret"
	service := NewService(mockRepo, secret)

	userID := uuid.New()
	token, err := jwtutil.GenerateToken(userID.String(), secret)
	assert.NoError(t, err)

	validUser := &User{
		ID:           userID,
		Username:     "tester",
		Email:        "test@mail.com",
		PasswordHash: "must-not-leak",
		Rating:       1320,
	}

	mockRepo.On("GetUserByID", userID).Return(validUser, nil)
	mockRepo.On("ListRatingsForUser", userID).Return([]UserRating{
		{
			UserID:      userID,
			Mode:        "classic",
			BoardSize:   8,
			TimeLimitMs: 300000,
			Rating:      1290,
			GamesPlayed: 4,
		},
	}, nil)

	profile, err := service.GetCurrentUser(token)

	assert.NoError(t, err)
	assert.Equal(t, userID.String(), profile.ID)
	assert.Equal(t, "tester", profile.Username)
	assert.Equal(t, "test@mail.com", profile.Email)
	assert.Equal(t, 1290, profile.Rating)
	assert.Len(t, profile.Ratings, 12)
	assert.Equal(t, 1290, profile.Ratings[1].Rating)
	assert.Equal(t, 4, profile.Ratings[1].GamesPlayed)
	mockRepo.AssertExpectations(t)
}

func TestService_GetCurrentUser_InvalidToken(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewService(mockRepo, "secret")

	profile, err := service.GetCurrentUser("not.a.valid.token")

	assert.Nil(t, profile)
	assert.Equal(t, ErrUnauthorized, err)
	mockRepo.AssertNotCalled(t, "GetUserByID", mock.Anything)
}

func TestService_GetCurrentUser_UserNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	secret := "secret"
	service := NewService(mockRepo, secret)

	userID := uuid.New()
	token, err := jwtutil.GenerateToken(userID.String(), secret)
	assert.NoError(t, err)

	mockRepo.On("GetUserByID", userID).Return(nil, ErrUserNotFound)

	profile, err := service.GetCurrentUser(token)

	assert.Nil(t, profile)
	assert.Equal(t, ErrUnauthorized, err)
	mockRepo.AssertExpectations(t)
}

func TestService_GetCurrentUser_DatabaseError(t *testing.T) {
	mockRepo := new(MockRepository)
	secret := "secret"
	service := NewService(mockRepo, secret)

	userID := uuid.New()
	token, err := jwtutil.GenerateToken(userID.String(), secret)
	assert.NoError(t, err)

	mockRepo.On("GetUserByID", userID).Return(nil, ErrDatabase)

	profile, err := service.GetCurrentUser(token)

	assert.Nil(t, profile)
	assert.Equal(t, ErrDatabase, err)
	mockRepo.AssertExpectations(t)
}

func TestService_GetCurrentUserRatings_FillsSupportedScopesWithDefaults(t *testing.T) {
	mockRepo := new(MockRepository)
	secret := "secret"
	service := NewService(mockRepo, secret)

	userID := uuid.New()
	token, err := jwtutil.GenerateToken(userID.String(), secret)
	assert.NoError(t, err)

	mockRepo.On("ListRatingsForUser", userID).Return([]UserRating{
		{
			UserID:      userID,
			Mode:        "classic",
			BoardSize:   10,
			TimeLimitMs: 600000,
			Rating:      1264,
			GamesPlayed: 2,
		},
	}, nil)

	ratings, err := service.GetCurrentUserRatings(token)

	assert.NoError(t, err)
	assert.Len(t, ratings, 12)
	assert.Equal(t, UserRatingDTO{
		RatingScopeDTO: RatingScopeDTO{
			Mode:             "classic",
			BoardSize:        8,
			TimeLimitMs:      60000,
			TimeLimitMinutes: 1,
		},
		Rating:      1200,
		GamesPlayed: 0,
	}, ratings[0])

	var foundScopedRating bool
	for _, rating := range ratings {
		if rating.BoardSize == 10 && rating.TimeLimitMinutes == 10 {
			foundScopedRating = true
			assert.Equal(t, 1264, rating.Rating)
			assert.Equal(t, 2, rating.GamesPlayed)
		}
	}
	assert.True(t, foundScopedRating)
	mockRepo.AssertExpectations(t)
}

func TestService_GetLeaderboard(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewService(mockRepo, "secret")

	user1ID := uuid.New()
	user2ID := uuid.New()
	scope := RatingScope{
		Mode:        "classic",
		BoardSize:   8,
		TimeLimitMs: 600000,
	}
	mockRepo.On("ListLeaderboard", scope, 50).Return([]LeaderboardEntry{
		{UserID: user1ID, Username: "leader", Rating: 1350, GamesPlayed: 7},
		{UserID: user2ID, Username: "runner", Rating: 1300, GamesPlayed: 3},
	}, nil)

	leaderboard, err := service.GetLeaderboard(scope, 50)

	assert.NoError(t, err)
	assert.Equal(t, RatingScopeDTO{
		Mode:             "classic",
		BoardSize:        8,
		TimeLimitMs:      600000,
		TimeLimitMinutes: 10,
	}, leaderboard.Scope)
	assert.Equal(t, []LeaderboardEntryDTO{
		{Rank: 1, UserID: user1ID.String(), Username: "leader", Rating: 1350, GamesPlayed: 7},
		{Rank: 2, UserID: user2ID.String(), Username: "runner", Rating: 1300, GamesPlayed: 3},
	}, leaderboard.Players)
	mockRepo.AssertExpectations(t)
}

func TestProfileSummaryRating(t *testing.T) {
	assert.Equal(t, 1320, profileSummaryRating(1320, []UserRatingDTO{
		{Rating: 1200, GamesPlayed: 0},
	}))

	assert.Equal(t, 1260, profileSummaryRating(1320, []UserRatingDTO{
		{Rating: 1260, GamesPlayed: 2},
		{Rating: 1230, GamesPlayed: 1},
		{Rating: 1400, GamesPlayed: 0},
	}))
}
