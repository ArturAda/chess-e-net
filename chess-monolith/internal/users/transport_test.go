package users

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockService для тестов транспорта
type MockService struct {
	mock.Mock
}

func (m *MockService) Register(username, email, password string) error {
	args := m.Called(username, email, password)
	return args.Error(0)
}

func (m *MockService) Login(email, password string) (string, error) {
	args := m.Called(email, password)
	return args.String(0), args.Error(1)
}

func TestHandler_Register_Scenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		request        RegisterRequest
		setupMock      func(mockService *MockService, request RegisterRequest)
		expectedStatus int
	}{
		{
			name: "Success",
			request: RegisterRequest{
				Username: "tester",
				Email:    "test@mail.com",
				Password: "password123",
			},
			setupMock: func(mockService *MockService, request RegisterRequest) {
				mockService.On("Register", request.Username, request.Email, request.Password).Return(nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "User Already Exists",
			request: RegisterRequest{
				Username: "tester",
				Email:    "exists@m.com",
				Password: "password",
			},
			setupMock: func(mockService *MockService, request RegisterRequest) {
				mockService.On("Register", request.Username, request.Email, request.Password).Return(ErrUserExists)
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name: "Internal Server Error",
			request: RegisterRequest{
				Username: "tester",
				Email:    "error@m.com",
				Password: "password",
			},
			setupMock: func(mockService *MockService, request RegisterRequest) {
				mockService.On("Register", request.Username, request.Email, request.Password).Return(ErrDatabase)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "Invalid Password Validation",
			request: RegisterRequest{
				Username: "tester",
				Email:    "bad-password@mail.com",
				Password: "bpswd",
			},
			setupMock: func(mockService *MockService, request RegisterRequest) {
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockService)
			tt.setupMock(mockService, tt.request)

			handler := NewHandler(mockService)
			router := gin.Default()
			handler.SetupRoutes(router)

			body, _ := json.Marshal(tt.request)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockService.AssertExpectations(t)
		})
	}
}

func TestHandler_Login_Scenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		request        LoginRequest
		setupMock      func(mockService *MockService, request LoginRequest)
		expectedStatus int
		expectedToken  string
	}{
		{
			name: "Success",
			request: LoginRequest{
				Email:    "test@mail.com",
				Password: "password123",
			},
			setupMock: func(mockService *MockService, request LoginRequest) {
				mockService.On("Login", request.Email, request.Password).Return("fake-jwt-token", nil)
			},
			expectedStatus: http.StatusOK,
			expectedToken:  "fake-jwt-token",
		},
		{
			name: "Invalid Credentials",
			request: LoginRequest{
				Email:    "test@mail.com",
				Password: "wrong-password",
			},
			setupMock: func(mockService *MockService, request LoginRequest) {
				mockService.On("Login", request.Email, request.Password).Return("", ErrInvalidCredentials)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Internal Server Error",
			request: LoginRequest{
				Email:    "test@mail.com",
				Password: "password123",
			},
			setupMock: func(mockService *MockService, request LoginRequest) {
				mockService.On("Login", request.Email, request.Password).Return("", ErrDatabase)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "Invalid Email Validation",
			request: LoginRequest{
				Email:    "bad-email",
				Password: "password",
			},
			setupMock: func(mockService *MockService, request LoginRequest) {
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockService)
			tt.setupMock(mockService, tt.request)

			handler := NewHandler(mockService)
			router := gin.Default()
			handler.SetupRoutes(router)

			body, _ := json.Marshal(tt.request)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedToken != "" {
				var resp map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				assert.Equal(t, tt.expectedToken, resp["token"])
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestHandler_Register_InvalidJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := new(MockService)
	handler := NewHandler(mockService)
	router := gin.Default()
	handler.SetupRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/register", bytes.NewBuffer([]byte("{bad-json")))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
