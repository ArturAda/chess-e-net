package users

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func (m *MockService) VerifyEmail(email, code string) error {
	args := m.Called(email, code)
	return args.Error(0)
}

func (m *MockService) ResendVerificationCode(email string) error {
	args := m.Called(email)
	return args.Error(0)
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

type fakeCaptchaVerifier struct {
	enabled bool
	siteKey string
	err     error
	token   string
}

func (v *fakeCaptchaVerifier) Enabled() bool { return v.enabled }
func (v *fakeCaptchaVerifier) SiteKey() string {
	return v.siteKey
}
func (v *fakeCaptchaVerifier) Verify(_ context.Context, token string, _ string) error {
	v.token = token
	return v.err
}

func TestHandler_Config_ReturnsTurnstilePublicSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandlerWithCaptcha(new(MockService), &fakeCaptchaVerifier{
		enabled: true,
		siteKey: "site-key",
	})
	router := gin.Default()
	handler.SetupRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/config", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Turnstile struct {
			Enabled bool   `json:"enabled"`
			SiteKey string `json:"site_key"`
		} `json:"turnstile"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Turnstile.Enabled)
	assert.Equal(t, "site-key", resp.Turnstile.SiteKey)
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

func TestHandler_Register_RequiresCaptchaWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockService)
	captcha := &fakeCaptchaVerifier{
		enabled: true,
		siteKey: "site-key",
		err:     ErrCaptchaRequired,
	}
	handler := NewHandlerWithCaptcha(mockService, captcha)
	router := gin.Default()
	handler.SetupRoutes(router)

	body, _ := json.Marshal(RegisterRequest{
		Username: "tester",
		Email:    "test@mail.com",
		Password: "password123",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, captcha.token)
	mockService.AssertNotCalled(t, "Register", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_Register_PassesCaptchaToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockService)
	mockService.On("Register", "tester", "test@mail.com", "password123").Return(nil)
	captcha := &fakeCaptchaVerifier{
		enabled: true,
		siteKey: "site-key",
	}
	handler := NewHandlerWithCaptcha(mockService, captcha)
	router := gin.Default()
	handler.SetupRoutes(router)

	body, _ := json.Marshal(RegisterRequest{
		Username:       "tester",
		Email:          "test@mail.com",
		Password:       "password123",
		TurnstileToken: "turnstile-token",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "turnstile-token", captcha.token)
	mockService.AssertExpectations(t)
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
			name: "Email Not Verified",
			request: LoginRequest{
				Email:    "test@mail.com",
				Password: "password123",
			},
			setupMock: func(mockService *MockService, request LoginRequest) {
				mockService.On("Login", request.Email, request.Password).Return("", ErrEmailNotVerified)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "Verification Email Delivery Failed",
			request: LoginRequest{
				Email:    "test@mail.com",
				Password: "password123",
			},
			setupMock: func(mockService *MockService, request LoginRequest) {
				mockService.On("Login", request.Email, request.Password).Return("", ErrEmailDelivery)
			},
			expectedStatus: http.StatusBadGateway,
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

func TestHandler_VerifyEmail_Scenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		request        VerifyEmailRequest
		setupMock      func(mockService *MockService, request VerifyEmailRequest)
		expectedStatus int
	}{
		{
			name: "Success",
			request: VerifyEmailRequest{
				Email: "test@mail.com",
				Code:  "123456",
			},
			setupMock: func(mockService *MockService, request VerifyEmailRequest) {
				mockService.On("VerifyEmail", request.Email, request.Code).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Invalid Code",
			request: VerifyEmailRequest{
				Email: "test@mail.com",
				Code:  "123456",
			},
			setupMock: func(mockService *MockService, request VerifyEmailRequest) {
				mockService.On("VerifyEmail", request.Email, request.Code).Return(ErrInvalidCode)
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Expired Code",
			request: VerifyEmailRequest{
				Email: "test@mail.com",
				Code:  "123456",
			},
			setupMock: func(mockService *MockService, request VerifyEmailRequest) {
				mockService.On("VerifyEmail", request.Email, request.Code).Return(ErrCodeExpired)
			},
			expectedStatus: http.StatusGone,
		},
		{
			name: "Invalid Body",
			request: VerifyEmailRequest{
				Email: "bad-email",
				Code:  "abc",
			},
			setupMock: func(mockService *MockService, request VerifyEmailRequest) {
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
			req, _ := http.NewRequest("POST", "/api/verify-email", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockService.AssertExpectations(t)
		})
	}
}

func TestHandler_ResendVerification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := new(MockService)
	mockService.On("ResendVerificationCode", "test@mail.com").Return(nil)

	handler := NewHandler(mockService)
	router := gin.Default()
	handler.SetupRoutes(router)

	body, _ := json.Marshal(ResendVerificationRequest{Email: "test@mail.com"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/resend-verification", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

func TestHandler_ResendVerification_ErrorScenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		body           []byte
		setupMock      func(*MockService)
		expectedStatus int
	}{
		{
			name:           "invalid body",
			body:           []byte(`{"email":"bad-email"}`),
			setupMock:      func(*MockService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "email delivery",
			body: []byte(`{"email":"test@mail.com"}`),
			setupMock: func(mockService *MockService) {
				mockService.On("ResendVerificationCode", "test@mail.com").Return(ErrEmailDelivery)
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name: "database",
			body: []byte(`{"email":"test@mail.com"}`),
			setupMock: func(mockService *MockService) {
				mockService.On("ResendVerificationCode", "test@mail.com").Return(ErrDatabase)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "generic error",
			body: []byte(`{"email":"test@mail.com"}`),
			setupMock: func(mockService *MockService) {
				mockService.On("ResendVerificationCode", "test@mail.com").Return(errors.New("unexpected"))
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
			req, _ := http.NewRequest(http.MethodPost, "/api/resend-verification", bytes.NewBuffer(tt.body))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
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

func TestHandler_Leaderboard_CustomScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := new(MockService)
	handler := NewHandler(mockService)
	router := gin.Default()
	handler.SetupRoutes(router)

	scope := RatingScope{
		Mode:        "custom",
		BoardSize:   9,
		TimeLimitMs: 300000,
	}
	mockService.On("GetLeaderboard", scope, 50).Return(&LeaderboardDTO{
		Scope:   ratingScopeDTO(scope),
		Players: []LeaderboardEntryDTO{},
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/leaderboard?mode=custom&board_size=9&time_limit=5", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

func TestHandler_Leaderboard_InvalidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := new(MockService)
	handler := NewHandler(mockService)
	router := gin.Default()
	handler.SetupRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/leaderboard?mode=classic&board_size=9&time_limit=0", nil)

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

func TestQueryHelperBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newQueryContext := func(rawQuery string) *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest(http.MethodGet, "/api/leaderboard?"+rawQuery, nil)
		c.Request = req
		return c
	}

	scope, err := ratingScopeFromQuery(newQueryContext(""))
	assert.NoError(t, err)
	assert.Equal(t, RatingScope{Mode: "classic", BoardSize: 8, TimeLimitMs: defaultRatingTimeLimitMs}, scope)

	scope, err = ratingScopeFromQuery(newQueryContext("mode=custom&board_size=9&time_limit_ms=123000"))
	assert.NoError(t, err)
	assert.Equal(t, RatingScope{Mode: "custom", BoardSize: 9, TimeLimitMs: 123000}, scope)

	_, err = ratingScopeFromQuery(newQueryContext("board_size=bad"))
	assert.EqualError(t, err, "board_size must be an integer")

	value, err := timeLimitMsFromQuery(newQueryContext("time_limit_minutes=5"))
	assert.NoError(t, err)
	assert.Equal(t, int64(300000), value)

	value, err = timeLimitMsFromQuery(newQueryContext("time_limit=30"))
	assert.NoError(t, err)
	assert.Equal(t, int64(1800000), value)

	_, err = timeLimitMsFromQuery(newQueryContext("time_limit_ms=0"))
	assert.EqualError(t, err, "time_limit_ms must be a positive integer")

	_, err = timeLimitMsFromQuery(newQueryContext("time_limit_minutes=bad"))
	assert.EqualError(t, err, "time_limit_minutes must be a positive integer")

	_, err = timeLimitMsFromQuery(newQueryContext("time_limit=0"))
	assert.EqualError(t, err, "time_limit must be a positive integer")

	limit, err := leaderboardLimitFromQuery(newQueryContext("limit=500"))
	assert.NoError(t, err)
	assert.Equal(t, 100, limit)

	_, err = leaderboardLimitFromQuery(newQueryContext("limit=0"))
	assert.EqualError(t, err, "limit must be positive")

	_, err = leaderboardLimitFromQuery(newQueryContext("limit=bad"))
	assert.EqualError(t, err, "limit must be an integer")

	boardSize, err := intQuery(newQueryContext(""), "board_size", 8)
	assert.NoError(t, err)
	assert.Equal(t, 8, boardSize)
}
