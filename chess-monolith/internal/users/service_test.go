package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
