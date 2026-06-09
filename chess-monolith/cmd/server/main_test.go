package main

import (
	"chess-monolith/internal/game/session"
	"chess-monolith/internal/users"
	"chess-monolith/internal/ws"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestServeDocumentationFileRendersAllowedDocsAndRejectsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	docsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "README.md"), []byte("# Docs\n\nHello **world**."), 0o644))
	t.Chdir(docsDir)

	router := gin.New()
	router.GET("/README.md", func(c *gin.Context) {
		serveDocumentationFile(c, "README.md")
	})
	router.GET("/missing", func(c *gin.Context) {
		serveDocumentationFile(c, "missing.md")
	})

	okRecorder := httptest.NewRecorder()
	okReq, err := http.NewRequest(http.MethodGet, "/README.md", nil)
	require.NoError(t, err)
	router.ServeHTTP(okRecorder, okReq)

	assert.Equal(t, http.StatusOK, okRecorder.Code)
	assert.Equal(t, "no-cache", okRecorder.Header().Get("Cache-Control"))
	assert.Contains(t, okRecorder.Body.String(), "<h1>Docs</h1>")
	assert.Contains(t, okRecorder.Body.String(), "<strong>world</strong>")

	missingRecorder := httptest.NewRecorder()
	missingReq, err := http.NewRequest(http.MethodGet, "/missing", nil)
	require.NoError(t, err)
	router.ServeHTTP(missingRecorder, missingReq)

	assert.Equal(t, http.StatusNotFound, missingRecorder.Code)
	assert.Contains(t, missingRecorder.Body.String(), "Not found")
}

func TestRenderDocumentationPageAndBody(t *testing.T) {
	markdown := strings.Join([]string{
		"<!-- hidden comment -->",
		"[![Coverage](badge.svg)](#)",
		"# Main Title",
		"Intro **strong** and `code` with [safe](https://example.com) and [bad](javascript:alert(1)).",
		"",
		"## Section",
		"* bullet one",
		"- bullet two",
		"1. ordered one",
		"> quoted text",
		"```",
		"<script>alert(1)</script>",
		"```",
		"continued paragraph",
		"on another line",
	}, "\n")

	body := renderDocumentationBody("README.md", []byte(markdown))
	assert.Contains(t, body, "<h1>Main Title</h1>")
	assert.Contains(t, body, "<strong>strong</strong>")
	assert.Contains(t, body, "<code>code</code>")
	assert.Contains(t, body, `<a href="https://example.com">safe</a>`)
	assert.Contains(t, body, "bad")
	assert.NotContains(t, body, `href="javascript:alert(1)"`)
	assert.Contains(t, body, "<ul><li>bullet one</li><li>bullet two</li></ul>")
	assert.Contains(t, body, "<ol><li>ordered one</li></ol>")
	assert.Contains(t, body, "<blockquote>quoted text</blockquote>")
	assert.Contains(t, body, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.Contains(t, body, "<p>continued paragraph on another line</p>")
	assert.NotContains(t, body, "hidden comment")
	assert.NotContains(t, body, "Coverage")

	licenseHTML := renderDocumentationBody("LICENSE", []byte("Line one\nline two\n\nLine three"))
	assert.Contains(t, licenseHTML, `<section class="license-text">`)
	assert.Contains(t, licenseHTML, "Line one<br>line two")
	assert.Contains(t, licenseHTML, "<p>Line three</p>")

	page := string(renderDocumentationPage("README.ru.md", []byte("# Русская документация")))
	assert.Contains(t, page, "<title>Документация Chess-E-Net</title>")
	assert.Contains(t, page, `<a href="/LICENSE">LICENSE</a>`)
	assert.Contains(t, page, "<h1>Русская документация</h1>")
}

func TestDocumentationInlineHelpers(t *testing.T) {
	assert.Equal(t, "Chess-E-Net documentation", documentationTitle("README.md"))
	assert.Equal(t, "Документация Chess-E-Net", documentationTitle("README.ru.md"))
	assert.Equal(t, "MIT License", documentationTitle("LICENSE"))
	assert.Equal(t, "Chess-E-Net documentation", documentationTitle("unknown"))

	level, text := markdownHeading("### Heading")
	assert.Equal(t, 3, level)
	assert.Equal(t, "Heading", text)
	level, text = markdownHeading("###NoSpace")
	assert.Equal(t, 0, level)
	assert.Empty(t, text)

	item, ok := unorderedListItem("* item")
	assert.True(t, ok)
	assert.Equal(t, "item", item)
	item, ok = orderedListItem("12. item")
	assert.True(t, ok)
	assert.Equal(t, "item", item)
	_, ok = orderedListItem("12.item")
	assert.False(t, ok)

	assert.True(t, isSafeDocumentationHref("https://example.com/docs"))
	assert.True(t, isSafeDocumentationHref("mailto:test@example.com"))
	assert.True(t, isSafeDocumentationHref("/README.md"))
	assert.False(t, isSafeDocumentationHref(""))
	assert.False(t, isSafeDocumentationHref("//example.com"))
	assert.False(t, isSafeDocumentationHref("../secret"))
	assert.False(t, isSafeDocumentationHref("javascript:alert(1)"))
	assert.False(t, isSafeDocumentationHref("bad\x00path"))

	assert.Equal(t, "a <code>b</code> c", replaceInlineCode("a `b` c"))
	assert.Equal(t, "a `b", replaceInlineCode("a `b"))
	assert.Equal(t, "a <strong>b</strong> c", replaceStrongText("a **b** c"))
	assert.Equal(t, "a **b", replaceStrongText("a **b"))
}

func TestDocumentationPathHelpers(t *testing.T) {
	assert.True(t, isAllowedDocumentationFile("README.md"))
	assert.True(t, isAllowedDocumentationFile("README.ru.md"))
	assert.True(t, isAllowedDocumentationFile("LICENSE"))
	assert.False(t, isAllowedDocumentationFile("configs/.env"))

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd", "server"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", "server", "main.go"), []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("docs"), 0o644))

	assert.True(t, isMonolithRoot(root))
	assert.False(t, isRepositoryRoot(root))
	assert.True(t, fileExists(filepath.Join(root, "README.md")))
	assert.False(t, fileExists(filepath.Join(root, "missing.md")))

	candidates := documentationFileCandidates("../README.md")
	for _, candidate := range candidates {
		assert.Equal(t, "README.md", filepath.Base(candidate))
	}

	paths := appendUniquePath(nil, " README.md ")
	paths = appendUniquePath(paths, "README.md")
	paths = appendUniquePath(paths, "")
	assert.Equal(t, []string{"README.md"}, paths)
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

func TestDocumentationAddressFromEnv(t *testing.T) {
	t.Setenv("DOCS_HOST", "")
	t.Setenv("DOCS_PORT", "")
	assert.Equal(t, "127.0.0.1:65000", documentationAddressFromEnv())

	t.Setenv("DOCS_HOST", "0.0.0.0")
	t.Setenv("DOCS_PORT", "65001")
	assert.Equal(t, "0.0.0.0:65001", documentationAddressFromEnv())
}

func TestRequestLimitAndEnvHelpers(t *testing.T) {
	t.Setenv("MAX_REQUEST_BODY_BYTES", "")
	assert.Equal(t, int64(1<<20), maxRequestBodyBytes())

	t.Setenv("MAX_REQUEST_BODY_BYTES", "2048")
	assert.Equal(t, int64(2048), maxRequestBodyBytes())

	t.Setenv("MAX_REQUEST_BODY_BYTES", "-1")
	assert.Equal(t, int64(1<<20), maxRequestBodyBytes())

	t.Setenv("CHESS_INT_TEST", "")
	assert.Equal(t, 7, intEnv("CHESS_INT_TEST", 7))
	t.Setenv("CHESS_INT_TEST", "12")
	assert.Equal(t, 12, intEnv("CHESS_INT_TEST", 7))
	t.Setenv("CHESS_INT_TEST", "bad")
	assert.Equal(t, 7, intEnv("CHESS_INT_TEST", 7))
}

func TestRateLimitHelpersCoverScopesAndCleanup(t *testing.T) {
	settings := rateLimitSettings{
		Window:             time.Minute,
		APIRequestsPerMin:  10,
		AuthRequestsPerMin: 2,
		WSRequestsPerMin:   3,
	}

	scope, limit := rateLimitScope("/api/login", settings)
	assert.Equal(t, "auth", scope)
	assert.Equal(t, 2, limit)
	scope, limit = rateLimitScope("/ws", settings)
	assert.Equal(t, "ws", scope)
	assert.Equal(t, 3, limit)
	scope, limit = rateLimitScope("/api/games", settings)
	assert.Equal(t, "api", scope)
	assert.Equal(t, 10, limit)
	scope, limit = rateLimitScope("/images/logo.png", settings)
	assert.Empty(t, scope)
	assert.Zero(t, limit)

	limiter := newIPRateLimiter(settings)
	now := time.Now()
	allowed, resetAt := limiter.allow("ip|auth", 1, now)
	assert.True(t, allowed)
	assert.True(t, resetAt.After(now))
	allowed, _ = limiter.allow("ip|auth", 1, now.Add(time.Second))
	assert.False(t, allowed)
	allowed, _ = limiter.allow("ip|auth", 1, now.Add(2*time.Minute))
	assert.True(t, allowed)
}

func TestFrontendCacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	setFrontendCacheHeaders(context, "assets/app.js")
	assert.Equal(t, "public, max-age=31536000, immutable", recorder.Header().Get("Cache-Control"))

	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	setFrontendCacheHeaders(context, "images/piece.png")
	assert.Equal(t, "public, max-age=86400", recorder.Header().Get("Cache-Control"))

	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	setFrontendCacheHeaders(context, "index.html")
	assert.Equal(t, "no-cache", recorder.Header().Get("Cache-Control"))
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
