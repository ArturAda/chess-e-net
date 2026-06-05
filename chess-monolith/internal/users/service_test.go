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

	profile, err := service.GetCurrentUser(token)

	assert.NoError(t, err)
	assert.Equal(t, userID.String(), profile.ID)
	assert.Equal(t, "tester", profile.Username)
	assert.Equal(t, "test@mail.com", profile.Email)
	assert.Equal(t, 1320, profile.Rating)
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
