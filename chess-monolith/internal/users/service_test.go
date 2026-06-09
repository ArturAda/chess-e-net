package users

import (
	"chess-monolith/pkg/jwtutil"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

func (m *MockRepository) UpdateEmailVerification(userID uuid.UUID, codeHash string, expiresAt time.Time) error {
	args := m.Called(userID, codeHash, expiresAt)
	return args.Error(0)
}

func (m *MockRepository) MarkEmailVerified(userID uuid.UUID, verifiedAt time.Time) error {
	args := m.Called(userID, verifiedAt)
	return args.Error(0)
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

type FakeEmailSender struct {
	sentTo []string
	codes  []string
	err    error
}

func (f *FakeEmailSender) SendVerificationCode(to string, code string) error {
	f.sentTo = append(f.sentTo, to)
	f.codes = append(f.codes, code)
	return f.err
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
	emailSender := &FakeEmailSender{}
	service := NewServiceWithEmailSender(mockRepo, "secret", emailSender)
	var createdUser *User

	// 1. Проверяем, что юзера нет (ожидаем ErrUserNotFound)
	mockRepo.On("GetUserByEmail", "new@mail.com").Return(nil, ErrUserNotFound)

	// 2. Ожидаем вызов CreateUser с любым объектом User
	mockRepo.On("CreateUser", mock.AnythingOfType("*users.User")).
		Run(func(args mock.Arguments) {
			createdUser = args.Get(0).(*User)
		}).
		Return(nil)

	err := service.Register("test", "new@mail.com", "password")

	assert.NoError(t, err)
	require.NotNil(t, createdUser)
	assert.Equal(t, "test", createdUser.Username)
	assert.Equal(t, "new@mail.com", createdUser.Email)
	assert.False(t, createdUser.EmailVerified)
	assert.NotEmpty(t, createdUser.EmailVerificationCodeHash)
	require.NotNil(t, createdUser.EmailVerificationExpiresAt)
	assert.WithinDuration(t, time.Now().UTC().Add(time.Minute), createdUser.EmailVerificationExpiresAt.UTC(), 5*time.Second)
	assert.Len(t, emailSender.sentTo, 1)
	assert.Equal(t, "new@mail.com", emailSender.sentTo[0])
	require.Len(t, emailSender.codes, 1)
	assert.True(t, verificationCodeMatches(createdUser.EmailVerificationCodeHash, emailSender.codes[0]))
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
		ID:            uuid.New(),
		Email:         "test@mail.com",
		PasswordHash:  string(hashedPassword),
		EmailVerified: true,
	}

	mockRepo.On("GetUserByEmail", "test@mail.com").Return(validUser, nil)

	token, err := service.Login("test@mail.com", "password123")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	mockRepo.AssertExpectations(t)
}

func TestService_Login_EmailNotVerified(t *testing.T) {
	mockRepo := new(MockRepository)
	emailSender := &FakeEmailSender{}
	service := NewServiceWithEmailSender(mockRepo, "secret", emailSender)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	userID := uuid.New()
	var updatedHash string
	var updatedExpiresAt time.Time

	validUser := &User{
		ID:            userID,
		Email:         "test@mail.com",
		PasswordHash:  string(hashedPassword),
		EmailVerified: false,
	}

	mockRepo.On("GetUserByEmail", "test@mail.com").Return(validUser, nil)
	mockRepo.On("UpdateEmailVerification", userID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
		Run(func(args mock.Arguments) {
			updatedHash = args.String(1)
			updatedExpiresAt = args.Get(2).(time.Time)
		}).
		Return(nil)

	token, err := service.Login("test@mail.com", "password123")

	assert.Empty(t, token)
	assert.Equal(t, ErrEmailNotVerified, err)
	assert.WithinDuration(t, time.Now().UTC().Add(time.Minute), updatedExpiresAt.UTC(), 5*time.Second)
	require.Len(t, emailSender.sentTo, 1)
	assert.Equal(t, "test@mail.com", emailSender.sentTo[0])
	require.Len(t, emailSender.codes, 1)
	assert.True(t, verificationCodeMatches(updatedHash, emailSender.codes[0]))
	mockRepo.AssertExpectations(t)
}

func TestService_Login_EmailNotVerifiedWrongPasswordDoesNotSendCode(t *testing.T) {
	mockRepo := new(MockRepository)
	emailSender := &FakeEmailSender{}
	service := NewServiceWithEmailSender(mockRepo, "secret", emailSender)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	userID := uuid.New()

	validUser := &User{
		ID:            userID,
		Email:         "test@mail.com",
		PasswordHash:  string(hashedPassword),
		EmailVerified: false,
	}

	mockRepo.On("GetUserByEmail", "test@mail.com").Return(validUser, nil)

	token, err := service.Login("test@mail.com", "wrong-password")

	assert.Empty(t, token)
	assert.Equal(t, ErrInvalidCredentials, err)
	assert.Empty(t, emailSender.sentTo)
	mockRepo.AssertNotCalled(t, "UpdateEmailVerification", mock.Anything, mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
}

func TestService_VerifyEmail_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewService(mockRepo, "secret")

	userID := uuid.New()
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	codeHash, err := hashVerificationCode("123456")
	require.NoError(t, err)

	mockRepo.On("GetUserByEmail", "test@mail.com").Return(&User{
		ID:                         userID,
		Email:                      "test@mail.com",
		EmailVerified:              false,
		EmailVerificationCodeHash:  codeHash,
		EmailVerificationExpiresAt: &expiresAt,
	}, nil)
	mockRepo.On("MarkEmailVerified", userID, mock.AnythingOfType("time.Time")).Return(nil)

	err = service.VerifyEmail("TEST@mail.com ", "123456")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_VerifyEmail_InvalidCode(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewService(mockRepo, "secret")

	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	codeHash, err := hashVerificationCode("123456")
	require.NoError(t, err)

	mockRepo.On("GetUserByEmail", "test@mail.com").Return(&User{
		ID:                         uuid.New(),
		Email:                      "test@mail.com",
		EmailVerified:              false,
		EmailVerificationCodeHash:  codeHash,
		EmailVerificationExpiresAt: &expiresAt,
	}, nil)

	err = service.VerifyEmail("test@mail.com", "654321")

	assert.Equal(t, ErrInvalidCode, err)
	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "MarkEmailVerified", mock.Anything, mock.Anything)
}

func TestService_VerifyEmail_ExpiredCode(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewService(mockRepo, "secret")

	expiresAt := time.Now().UTC().Add(-time.Minute)
	codeHash, err := hashVerificationCode("123456")
	require.NoError(t, err)

	mockRepo.On("GetUserByEmail", "test@mail.com").Return(&User{
		ID:                         uuid.New(),
		Email:                      "test@mail.com",
		EmailVerified:              false,
		EmailVerificationCodeHash:  codeHash,
		EmailVerificationExpiresAt: &expiresAt,
	}, nil)

	err = service.VerifyEmail("test@mail.com", "123456")

	assert.Equal(t, ErrCodeExpired, err)
	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "MarkEmailVerified", mock.Anything, mock.Anything)
}

func TestService_ResendVerificationCode_UpdatesHashAndSends(t *testing.T) {
	mockRepo := new(MockRepository)
	emailSender := &FakeEmailSender{}
	service := NewServiceWithEmailSender(mockRepo, "secret", emailSender)

	userID := uuid.New()
	var updatedHash string

	mockRepo.On("GetUserByEmail", "test@mail.com").Return(&User{
		ID:            userID,
		Email:         "test@mail.com",
		EmailVerified: false,
	}, nil)
	mockRepo.On("UpdateEmailVerification", userID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
		Run(func(args mock.Arguments) {
			updatedHash = args.String(1)
		}).
		Return(nil)

	err := service.ResendVerificationCode("test@mail.com")

	assert.NoError(t, err)
	require.Len(t, emailSender.sentTo, 1)
	assert.Equal(t, "test@mail.com", emailSender.sentTo[0])
	require.Len(t, emailSender.codes, 1)
	assert.True(t, verificationCodeMatches(updatedHash, emailSender.codes[0]))
	mockRepo.AssertExpectations(t)
}

func TestService_ResendVerificationCode_NoopsAndErrors(t *testing.T) {
	t.Run("empty email is ignored", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewServiceWithEmailSender(mockRepo, "secret", &FakeEmailSender{})

		err := service.ResendVerificationCode("   ")

		assert.NoError(t, err)
		mockRepo.AssertNotCalled(t, "GetUserByEmail", mock.Anything)
	})

	t.Run("unknown email is ignored", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewServiceWithEmailSender(mockRepo, "secret", &FakeEmailSender{})
		mockRepo.On("GetUserByEmail", "missing@mail.com").Return(nil, ErrUserNotFound)

		err := service.ResendVerificationCode("missing@mail.com")

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("verified user is ignored", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewServiceWithEmailSender(mockRepo, "secret", &FakeEmailSender{})
		mockRepo.On("GetUserByEmail", "verified@mail.com").Return(&User{
			ID:            uuid.New(),
			Email:         "verified@mail.com",
			EmailVerified: true,
		}, nil)

		err := service.ResendVerificationCode("verified@mail.com")

		assert.NoError(t, err)
		mockRepo.AssertNotCalled(t, "UpdateEmailVerification", mock.Anything, mock.Anything, mock.Anything)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository lookup error is returned", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewServiceWithEmailSender(mockRepo, "secret", &FakeEmailSender{})
		mockRepo.On("GetUserByEmail", "error@mail.com").Return(nil, ErrDatabase)

		err := service.ResendVerificationCode("error@mail.com")

		assert.Equal(t, ErrDatabase, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("empty stored email blocks delivery", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewServiceWithEmailSender(mockRepo, "secret", &FakeEmailSender{})
		mockRepo.On("GetUserByEmail", "empty@mail.com").Return(&User{
			ID:            uuid.New(),
			Email:         "   ",
			EmailVerified: false,
		}, nil)

		err := service.ResendVerificationCode("empty@mail.com")

		assert.Equal(t, ErrEmailDelivery, err)
		mockRepo.AssertNotCalled(t, "UpdateEmailVerification", mock.Anything, mock.Anything, mock.Anything)
		mockRepo.AssertExpectations(t)
	})

	t.Run("update error is returned before email delivery", func(t *testing.T) {
		mockRepo := new(MockRepository)
		emailSender := &FakeEmailSender{}
		service := NewServiceWithEmailSender(mockRepo, "secret", emailSender)
		userID := uuid.New()
		mockRepo.On("GetUserByEmail", "test@mail.com").Return(&User{
			ID:            userID,
			Email:         "test@mail.com",
			EmailVerified: false,
		}, nil)
		mockRepo.On("UpdateEmailVerification", userID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
			Return(ErrDatabase)

		err := service.ResendVerificationCode("test@mail.com")

		assert.Equal(t, ErrDatabase, err)
		assert.Empty(t, emailSender.sentTo)
		mockRepo.AssertExpectations(t)
	})

	t.Run("email delivery error is returned", func(t *testing.T) {
		mockRepo := new(MockRepository)
		emailSender := &FakeEmailSender{err: ErrEmailDelivery}
		service := NewServiceWithEmailSender(mockRepo, "secret", emailSender)
		userID := uuid.New()
		mockRepo.On("GetUserByEmail", "test@mail.com").Return(&User{
			ID:            userID,
			Email:         "test@mail.com",
			EmailVerified: false,
		}, nil)
		mockRepo.On("UpdateEmailVerification", userID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
			Return(nil)

		err := service.ResendVerificationCode("test@mail.com")

		assert.Equal(t, ErrEmailDelivery, err)
		assert.Equal(t, []string{"test@mail.com"}, emailSender.sentTo)
		mockRepo.AssertExpectations(t)
	})
}

func TestService_RefreshVerificationCodeDirectNoops(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewServiceWithEmailSender(mockRepo, "secret", &FakeEmailSender{}).(*service)

	assert.NoError(t, service.refreshVerificationCode(nil))
	assert.NoError(t, service.refreshVerificationCode(&User{EmailVerified: true}))
	mockRepo.AssertNotCalled(t, "UpdateEmailVerification", mock.Anything, mock.Anything, mock.Anything)
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
		{
			UserID:      userID,
			Mode:        "custom",
			BoardSize:   9,
			TimeLimitMs: 180000,
			Rating:      1335,
			GamesPlayed: 2,
		},
	}, nil)

	profile, err := service.GetCurrentUser(token)

	assert.NoError(t, err)
	assert.Equal(t, userID.String(), profile.ID)
	assert.Equal(t, "tester", profile.Username)
	assert.Equal(t, "test@mail.com", profile.Email)
	assert.Equal(t, 1335, profile.Rating)
	assert.Len(t, profile.Ratings, 13)
	assert.Equal(t, 1290, profile.Ratings[1].Rating)
	assert.Equal(t, 4, profile.Ratings[1].GamesPlayed)

	var foundCustomRating bool
	for _, rating := range profile.Ratings {
		if rating.Mode == "custom" && rating.BoardSize == 9 && rating.TimeLimitMinutes == 3 {
			foundCustomRating = true
			assert.Equal(t, 1335, rating.Rating)
			assert.Equal(t, 2, rating.GamesPlayed)
		}
	}
	assert.True(t, foundCustomRating)
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
