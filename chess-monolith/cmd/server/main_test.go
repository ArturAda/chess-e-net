package main

import (
	"chess-monolith/internal/game/session"
	"chess-monolith/internal/users"
	"chess-monolith/internal/ws"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DummyUserRepository struct{}

func (d *DummyUserRepository) CreateUser(_ *users.User) error               { return nil }
func (d *DummyUserRepository) GetUserByEmail(_ string) (*users.User, error) { return nil, nil }
func (d *DummyUserRepository) GetUserByID(_ uuid.UUID) (*users.User, error) {
	return nil, nil
}
func (d *DummyUserRepository) UpdateEmailVerification(_ uuid.UUID, _ string, _ time.Time) error {
	return nil
}
func (d *DummyUserRepository) MarkEmailVerified(_ uuid.UUID, _ time.Time) error {
	return nil
}
func (d *DummyUserRepository) UpdateRatings(_, _ uuid.UUID, _, _ int) error {
	return nil
}
func (d *DummyUserRepository) GetOrCreateRating(_ uuid.UUID, _ users.RatingScope) (*users.UserRating, error) {
	return &users.UserRating{Rating: users.DefaultRating}, nil
}
func (d *DummyUserRepository) ListRatingsForUser(_ uuid.UUID) ([]users.UserRating, error) {
	return nil, nil
}
func (d *DummyUserRepository) ListLeaderboard(_ users.RatingScope, _ int) ([]users.LeaderboardEntry, error) {
	return nil, nil
}
func (d *DummyUserRepository) ApplyRatingResult(_ uuid.UUID, _ uuid.UUID, _ users.RatingScope, _ float64) (int, int, error) {
	return 1216, 1184, nil
}

type DummyQueueManager struct{}

func (d *DummyQueueManager) AddPlayer(_ *ws.Client, _ string, _ int, _ bool, _ time.Duration) error {
	return nil
}
func (d *DummyQueueManager) RemovePlayer(_ *ws.Client) {}

func TestInitGameRegistryIncludesOnlineBoardSizes(t *testing.T) {
	registry := initGameRegistry()

	tests := []struct {
		modeName string
		size     int
	}{
		{modeName: "classic", size: 8},
		{modeName: "modern10", size: 10},
		{modeName: "modern12", size: 12},
	}

	for _, tt := range tests {
		t.Run(tt.modeName, func(t *testing.T) {
			s, err := session.NewSession(registry, tt.modeName, 10*time.Minute)

			require.NoError(t, err)
			assert.Equal(t, tt.size, s.Board.Width)
			assert.Equal(t, tt.size, s.Board.Height)
		})
	}
}

func TestPingRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var userHandler *users.Handler = nil
	hub := ws.NewHub()
	dummyRepo := &DummyUserRepository{}
	dummyQM := &DummyQueueManager{}

	router := SetupRouter(userHandler, hub, dummyRepo, "test-secret", dummyQM, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/ping", nil)
	assert.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
	assert.Contains(t, w.Body.String(), "pong")
}

func TestSecurityHeadersAreSet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := SetupRouter(nil, ws.NewHub(), &DummyUserRepository{}, "test-secret", &DummyQueueManager{}, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/ping", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Contains(t, w.Header().Get("Content-Security-Policy"), "object-src 'none'")
}

func TestCORSUsesAllowedOriginsOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ALLOWED_ORIGINS", "http://allowed.test")

	router := SetupRouter(nil, ws.NewHub(), &DummyUserRepository{}, "test-secret", &DummyQueueManager{}, nil)

	allowed := httptest.NewRecorder()
	allowedReq, err := http.NewRequest("GET", "/api/ping", nil)
	require.NoError(t, err)
	allowedReq.Header.Set("Origin", "http://allowed.test")
	router.ServeHTTP(allowed, allowedReq)

	sameOrigin := httptest.NewRecorder()
	sameOriginReq, err := http.NewRequest("GET", "/api/ping", nil)
	require.NoError(t, err)
	sameOriginReq.Host = "192.168.1.10:8080"
	sameOriginReq.Header.Set("Origin", "http://192.168.1.10:8080")
	router.ServeHTTP(sameOrigin, sameOriginReq)

	rejected := httptest.NewRecorder()
	rejectedReq, err := http.NewRequest("GET", "/api/ping", nil)
	require.NoError(t, err)
	rejectedReq.Header.Set("Origin", "http://evil.test")
	router.ServeHTTP(rejected, rejectedReq)

	assert.Equal(t, "http://allowed.test", allowed.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, http.StatusOK, sameOrigin.Code)
	assert.NotEqual(t, "*", sameOrigin.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rejected.Header().Get("Access-Control-Allow-Origin"))
}

func TestFrontendStaticKeepsRequestsInsideDist(t *testing.T) {
	distDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(distDir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("index"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "assets", "app.js"), []byte("console.log('ok')"), 0o644))

	staticFile, requestedPath, ok := safeFrontendFilePath(distDir, "/assets/app.js")
	require.True(t, ok)
	assert.Equal(t, "assets/app.js", requestedPath)
	assert.Equal(t, filepath.Join(distDir, "assets", "app.js"), staticFile)

	_, _, ok = safeFrontendFilePath(distDir, "/../secret.txt")
	assert.False(t, ok)
	assert.True(t, isSuspiciousFrontendPath("/../secret.txt"))
	assert.True(t, isSuspiciousFrontendPath("/assets/../../secret.txt"))
}

func TestFrontendStaticRejectsTraversalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	distDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("index"), 0o644))

	router := gin.New()
	setupFrontendStatic(router, distDir)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/assets/../../secret.txt", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), "index")
}

func TestFrontendStaticRejectsSymlinkOutsideDist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	distDir := t.TempDir()
	outsideDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(distDir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("index"), 0o644))
	secretFile := filepath.Join(outsideDir, "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("secret"), 0o644))

	linkPath := filepath.Join(distDir, "assets", "leak.txt")
	if err := os.Symlink(secretFile, linkPath); err != nil {
		t.Skipf("symlink is not available on this filesystem: %v", err)
	}

	router := gin.New()
	setupFrontendStatic(router, distDir)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/assets/leak.txt", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), "secret")
}

func TestFrontendStaticFindsNestedDistFromRepositoryRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rootDir := t.TempDir()
	distDir := filepath.Join(rootDir, "chess-monolith", "frontend", "dist")
	require.NoError(t, os.MkdirAll(distDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("nested index"), 0o644))
	t.Chdir(rootDir)

	router := gin.New()
	setupFrontendStatic(router, "")

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "nested index")
}

func TestFrontendStaticServesAssetsFromRelativeDistDir(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rootDir := t.TempDir()
	distDir := filepath.Join(rootDir, "frontend", "dist")
	require.NoError(t, os.MkdirAll(filepath.Join(distDir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("index"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "assets", "app.css"), []byte("body{}"), 0o644))
	t.Chdir(rootDir)

	router := gin.New()
	setupFrontendStatic(router, "frontend/dist")

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/assets/app.css", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "body{}", w.Body.String())
}

func TestLoadDotEnvFindsNestedConfigFromRepositoryRoot(t *testing.T) {
	rootDir := t.TempDir()
	configDir := filepath.Join(rootDir, "chess-monolith", "configs")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	key := "CHESS_TEST_NESTED_ENV"
	require.NoError(t, os.Unsetenv(key))
	defer os.Unsetenv(key)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, ".env"), []byte(key+"=loaded\n"), 0o644))
	t.Chdir(rootDir)

	loadDotEnv()

	assert.Equal(t, "loaded", os.Getenv(key))
}

func TestReadDocumentationFileSupportsRuntimeDocsDirectory(t *testing.T) {
	docsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "README.md"), []byte("english docs"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "README.ru.md"), []byte("russian docs"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "LICENSE"), []byte("license text"), 0o644))
	t.Chdir(docsDir)

	content, path, err := readDocumentationFile("LICENSE")

	require.NoError(t, err)
	assert.Equal(t, []byte("license text"), content)
	assert.Equal(t, filepath.Join(docsDir, "LICENSE"), path)
}

func TestGameAddressFromEnvDefaultsToLocalhost(t *testing.T) {
	t.Setenv("HOST", "")
	t.Setenv("PORT", "")

	assert.Equal(t, "127.0.0.1:8080", gameAddressFromEnv())
}

func TestGameAddressFromEnvUsesConfiguredHostAndPort(t *testing.T) {
	t.Setenv("HOST", "0.0.0.0")
	t.Setenv("PORT", "8080")

	assert.Equal(t, "0.0.0.0:8080", gameAddressFromEnv())
}

func TestRateLimitBlocksAuthBurst(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("AUTH_RATE_LIMIT_REQUESTS_PER_MINUTE", "2")

	router := SetupRouter(nil, ws.NewHub(), &DummyUserRepository{}, "test-secret", &DummyQueueManager{}, nil)

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", "/api/login", nil)
		require.NoError(t, err)
		router.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusTooManyRequests, w.Code)
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/api/login", nil)
	require.NoError(t, err)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestSetupRouter_FullFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := ws.NewHub()
	dummyRepo := &DummyUserRepository{}
	dummyQM := &DummyQueueManager{}

	var userHandler *users.Handler = nil

	router := SetupRouter(userHandler, hub, dummyRepo, "test-secret", dummyQM, nil)

	// Проверяем, что роутер поднялся и эндпоинты зарегистрированы
	assert.NotNil(t, router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws", nil)
	router.ServeHTTP(w, req)

	// Ожидаем 401, так как нет токена (значит роут /ws работает и просит авторизацию)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
