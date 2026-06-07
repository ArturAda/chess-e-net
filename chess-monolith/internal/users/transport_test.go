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

func (m *MockService) GetCurrentUser(token string) (*UserProfile, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UserProfile), args.Error(1)
}

func (m *MockService) GetCurrentUserRatings(token string) ([]UserRatingDTO, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]UserRatingDTO), args.Error(1)
}

func (m *MockService) GetLeaderboard(scope RatingScope, limit int) (*LeaderboardDTO, error) {
	args := m.Called(scope, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*LeaderboardDTO), args.Error(1)
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

func TestHandler_Me_Scenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		authHeader     string
		setupMock      func(mockService *MockService)
		expectedStatus int
		assertBody     func(t *testing.T, body []byte)
	}{
		{
			name:       "Success",
			authHeader: "Bearer valid-token",
			setupMock: func(mockService *MockService) {
				mockService.On("GetCurrentUser", "valid-token").Return(&UserProfile{
					ID:       "550e8400-e29b-41d4-a716-446655440000",
					Username: "tester",
					Email:    "test@mail.com",
					Rating:   1200,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, body []byte) {
				var resp map[string]any
				err := json.Unmarshal(body, &resp)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", resp["id"])
				assert.Equal(t, "tester", resp["username"])
				assert.Equal(t, "test@mail.com", resp["email"])
				assert.Equal(t, float64(1200), resp["rating"])
				assert.NotContains(t, resp, "password")
				assert.NotContains(t, resp, "password_hash")
				assert.NotContains(t, resp, "PasswordHash")
			},
		},
		{
			name:       "Missing Authorization Header",
			authHeader: "",
			setupMock: func(mockService *MockService) {
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "Invalid Authorization Scheme",
			authHeader: "Token valid-token",
			setupMock: func(mockService *MockService) {
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "Invalid Token",
			authHeader: "Bearer invalid-token",
			setupMock: func(mockService *MockService) {
				mockService.On("GetCurrentUser", "invalid-token").Return(nil, ErrUnauthorized)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "Database Error",
			authHeader: "Bearer valid-token",
			setupMock: func(mockService *MockService) {
				mockService.On("GetCurrentUser", "valid-token").Return(nil, ErrDatabase)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockService)
			tt.setupMock(mockService)

			handler := NewHandler(mockService)
			router := gin.Default()
			handler.SetupRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/me", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.assertBody != nil {
				tt.assertBody(t, w.Body.Bytes())
			}
			mockService.AssertExpectations(t)
		})
	}
}

func TestHandler_MeRatings_Scenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		authHeader     string
		setupMock      func(mockService *MockService)
		expectedStatus int
		assertBody     func(t *testing.T, body []byte)
	}{
		{
			name:       "Success",
			authHeader: "Bearer valid-token",
			setupMock: func(mockService *MockService) {
				mockService.On("GetCurrentUserRatings", "valid-token").Return([]UserRatingDTO{
					{
						RatingScopeDTO: RatingScopeDTO{
							Mode:             "classic",
							BoardSize:        8,
							TimeLimitMs:      600000,
							TimeLimitMinutes: 10,
						},
						Rating:      1234,
						GamesPlayed: 5,
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, body []byte) {
				var resp map[string][]UserRatingDTO
				err := json.Unmarshal(body, &resp)
				if err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				assert.Len(t, resp["ratings"], 1)
				assert.Equal(t, 1234, resp["ratings"][0].Rating)
				assert.Equal(t, 8, resp["ratings"][0].BoardSize)
				assert.Equal(t, 10, resp["ratings"][0].TimeLimitMinutes)
			},
		},
		{
			name:       "Missing Authorization Header",
			authHeader: "",
			setupMock: func(mockService *MockService) {
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "Invalid Token",
			authHeader: "Bearer invalid-token",
			setupMock: func(mockService *MockService) {
				mockService.On("GetCurrentUserRatings", "invalid-token").Return(nil, ErrUnauthorized)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "Database Error",
			authHeader: "Bearer valid-token",
			setupMock: func(mockService *MockService) {
				mockService.On("GetCurrentUserRatings", "valid-token").Return(nil, ErrDatabase)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockService)
			tt.setupMock(mockService)

			handler := NewHandler(mockService)
			router := gin.Default()
			handler.SetupRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/me/ratings", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.assertBody != nil {
				tt.assertBody(t, w.Body.Bytes())
			}
			mockService.AssertExpectations(t)
		})
	}
}

func TestHandler_Leaderboard_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := new(MockService)
	handler := NewHandler(mockService)
	router := gin.Default()
	handler.SetupRoutes(router)

	scope := RatingScope{
		Mode:        "classic",
		BoardSize:   10,
		TimeLimitMs: 300000,
	}
	mockService.On("GetLeaderboard", scope, 25).Return(&LeaderboardDTO{
		Scope: ratingScopeDTO(scope),
		Players: []LeaderboardEntryDTO{
			{
				Rank:        1,
				UserID:      "550e8400-e29b-41d4-a716-446655440000",
				Username:    "leader",
				Rating:      1400,
				GamesPlayed: 8,
			},
		},
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/leaderboard?mode=classic&board_size=10&time_limit=5&limit=25", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp LeaderboardDTO
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 10, resp.Scope.BoardSize)
	assert.Equal(t, 5, resp.Scope.TimeLimitMinutes)
	assert.Len(t, resp.Players, 1)
	assert.Equal(t, "leader", resp.Players[0].Username)
	mockService.AssertExpectations(t)
}

func TestHandler_Leaderboard_InvalidScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := new(MockService)
	handler := NewHandler(mockService)
	router := gin.Default()
	handler.SetupRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/leaderboard?mode=classic&board_size=9&time_limit=5", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockService.AssertExpectations(t)
}

func TestHandler_Leaderboard_DatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := new(MockService)
	handler := NewHandler(mockService)
	router := gin.Default()
	handler.SetupRoutes(router)

	scope := RatingScope{
		Mode:        "classic",
		BoardSize:   8,
		TimeLimitMs: 600000,
	}
	mockService.On("GetLeaderboard", scope, 50).Return(nil, ErrDatabase)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/leaderboard", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}

func TestBearerTokenFromHeader(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{name: "valid", header: "Bearer token-value", wantToken: "token-value", wantOK: true},
		{name: "valid with spaces", header: "Bearer   token-value   ", wantToken: "token-value", wantOK: true},
		{name: "empty", header: "", wantOK: false},
		{name: "wrong scheme", header: "Token token-value", wantOK: false},
		{name: "missing token", header: "Bearer ", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := bearerTokenFromHeader(tt.header)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}
