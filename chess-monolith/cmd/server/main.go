package main

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"chess-monolith/internal/game"
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/modes/classic"
	"chess-monolith/internal/game/modes/modern"
	"chess-monolith/internal/matchmaking"
	"chess-monolith/internal/users"
	"chess-monolith/internal/ws"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// initDB настраивает и возвращает подключение к PostgreSQL
func initDB(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	log.Println("Connected to database")
	return db
}

// initGameRegistry собирает реестр правил и регистрирует все доступные режимы
func initGameRegistry() *core.Registry {
	registry := core.NewRegistry()
	classic.Register(registry) // Добавляем классические шахматы
	modern.Register(registry)  // Добавляем 10x10 и 12x12 online modes
	// TODO В будущем добавить: chess960.Register(registry) и т.д.
	return registry
}

// SetupRouter настраивает middlewares и эндпоинты
func SetupRouter(userHandler *users.Handler, hub *ws.Hub, userRepo users.Repository, jwtSecret string, qm ws.QueueManager, gameRepo game.Repository) *gin.Engine {
	router := gin.Default()
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Printf("Unable to disable trusted proxies: %v", err)
	}
	router.MaxMultipartMemory = maxRequestBodyBytes()

	router.Use(securityHeaders())
	router.Use(cors.New(corsConfig()))
	router.Use(requestBodyLimit(maxRequestBodyBytes()))
	router.Use(newIPRateLimiter(rateLimitSettingsFromEnv()).Middleware())

	router.GET("/api/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "pong"})
	})

	router.GET("/ws", ws.ServeWS(hub, userRepo, jwtSecret, qm))

	if userHandler != nil {
		userHandler.SetupRoutes(router)
	}
	if gameRepo != nil && userRepo != nil {
		game.NewHandler(gameRepo, userRepo, jwtSecret).SetupRoutes(router)
	}

	setupFrontendStatic(router, os.Getenv("FRONTEND_DIST_DIR"))

	return router
}

func setupFrontendStatic(router *gin.Engine, distDir string) {
	distDir, indexFile, ok := resolveFrontendDistDir(distDir)
	if !ok {
		log.Printf("Frontend static files disabled: %s not found", indexFile)
		return
	}

	router.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == "/api" || strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.URL.Path == "/ws" || strings.HasPrefix(c.Request.URL.Path, "/ws/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		if isSuspiciousFrontendPath(c.Request.URL.Path) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		if staticFile, requestedPath, ok := safeFrontendFilePath(distDir, c.Request.URL.Path); ok {
			if info, err := os.Stat(staticFile); err == nil && !info.IsDir() {
				if !frontendFileInsideDist(distDir, staticFile) {
					c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
					return
				}
				setFrontendCacheHeaders(c, requestedPath)
				c.File(staticFile)
				return
			}
		}

		c.Header("Cache-Control", "no-cache")
		c.File(indexFile)
	})
}

func resolveFrontendDistDir(distDir string) (string, string, bool) {
	candidates := frontendDistDirCandidates(distDir)
	for _, candidate := range candidates {
		indexFile := filepath.Join(candidate, "index.html")
		if _, err := os.Stat(indexFile); err == nil {
			return candidate, indexFile, true
		}
	}

	fallback := "frontend/dist"
	if len(candidates) > 0 {
		fallback = candidates[0]
	}
	return fallback, filepath.Join(fallback, "index.html"), false
}

func frontendDistDirCandidates(distDir string) []string {
	candidates := make([]string, 0, 4)
	if strings.TrimSpace(distDir) != "" {
		candidates = appendUniquePath(candidates, distDir)
	}
	candidates = appendUniquePath(candidates, "frontend/dist")
	candidates = appendUniquePath(candidates, "chess-monolith/frontend/dist")

	if executable, err := os.Executable(); err == nil {
		candidates = appendUniquePath(candidates, filepath.Join(filepath.Dir(executable), "frontend", "dist"))
	}

	return candidates
}

func appendUniquePath(paths []string, next string) []string {
	next = filepath.Clean(strings.TrimSpace(next))
	if next == "." || next == "" {
		return paths
	}
	for _, existing := range paths {
		if existing == next {
			return paths
		}
	}
	return append(paths, next)
}

func setFrontendCacheHeaders(c *gin.Context, requestedPath string) {
	path := strings.ReplaceAll(requestedPath, string(filepath.Separator), "/")
	switch {
	case strings.HasPrefix(path, "assets/"):
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	case strings.HasPrefix(path, "images/"):
		c.Header("Cache-Control", "public, max-age=86400")
	default:
		c.Header("Cache-Control", "no-cache")
	}
}

func safeFrontendFilePath(distDir string, requestPath string) (string, string, bool) {
	if isSuspiciousFrontendPath(requestPath) {
		return "", "", false
	}

	cleanPath := filepath.Clean("/" + strings.TrimPrefix(requestPath, "/"))
	requestedPath := strings.TrimPrefix(cleanPath, string(filepath.Separator))
	if requestedPath == "." || requestedPath == "" {
		return "", "", false
	}

	distAbs, err := filepath.Abs(distDir)
	if err != nil {
		return "", "", false
	}

	staticFile, err := filepath.Abs(filepath.Join(distAbs, requestedPath))
	if err != nil {
		return "", "", false
	}

	relativePath, err := filepath.Rel(distAbs, staticFile)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || filepath.IsAbs(relativePath) {
		return "", "", false
	}

	return staticFile, filepath.ToSlash(relativePath), true
}

func frontendFileInsideDist(distDir string, staticFile string) bool {
	distAbs, err := filepath.Abs(distDir)
	if err != nil {
		return false
	}

	distReal, err := filepath.EvalSymlinks(distAbs)
	if err != nil {
		return false
	}

	staticAbs, err := filepath.Abs(staticFile)
	if err != nil {
		return false
	}

	fileReal, err := filepath.EvalSymlinks(staticAbs)
	if err != nil {
		return false
	}

	relativePath, err := filepath.Rel(distReal, fileReal)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || filepath.IsAbs(relativePath) {
		return false
	}

	return true
}

func isSuspiciousFrontendPath(requestPath string) bool {
	if strings.ContainsRune(requestPath, 0) {
		return true
	}

	normalized := strings.ReplaceAll(requestPath, "\\", "/")
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return true
		}
	}

	return false
}

func corsConfig() cors.Config {
	config := cors.DefaultConfig()
	allowedOrigins := allowedOriginsFromEnv()
	config.AllowOriginWithContextFunc = func(c *gin.Context, origin string) bool {
		return originAllowedForRequest(origin, c.Request.Host, allowedOrigins)
	}
	config.AllowMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	config.ExposeHeaders = []string{"Content-Length"}
	config.MaxAge = 12 * time.Hour
	return config
}

func allowedOriginsFromEnv() []string {
	origins := splitCSV(os.Getenv("ALLOWED_ORIGINS"))
	if len(origins) > 0 {
		return origins
	}

	return []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:8080",
		"http://127.0.0.1:8080",
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func originAllowedForRequest(origin string, requestHost string, allowedOrigins []string) bool {
	originURL, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || originURL.Scheme == "" || originURL.Host == "" {
		return false
	}

	if strings.EqualFold(originURL.Host, requestHost) {
		return true
	}

	originKey := canonicalHTTPOrigin(originURL)
	for _, allowedOrigin := range allowedOrigins {
		allowedURL, err := url.Parse(strings.TrimSpace(allowedOrigin))
		if err == nil && canonicalHTTPOrigin(allowedURL) == originKey {
			return true
		}
	}

	return false
}

func canonicalHTTPOrigin(originURL *url.URL) string {
	if originURL == nil || originURL.Scheme == "" || originURL.Host == "" {
		return ""
	}

	return strings.ToLower(originURL.Scheme) + "://" + strings.ToLower(originURL.Host)
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", strings.Join([]string{
			"default-src 'self'",
			"script-src 'self' 'unsafe-inline' https://challenges.cloudflare.com",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data:",
			"connect-src 'self' ws: wss:",
			"frame-src https://challenges.cloudflare.com",
			"font-src 'self'",
			"object-src 'none'",
			"base-uri 'self'",
			"frame-ancestors 'none'",
		}, "; "))
		c.Next()
	}
}

func requestBodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func maxRequestBodyBytes() int64 {
	const fallback = int64(1 << 20)
	raw := strings.TrimSpace(os.Getenv("MAX_REQUEST_BODY_BYTES"))
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		log.Printf("Invalid MAX_REQUEST_BODY_BYTES=%q, using %d", raw, fallback)
		return fallback
	}

	return value
}

type rateLimitSettings struct {
	Window             time.Duration
	APIRequestsPerMin  int
	AuthRequestsPerMin int
	WSRequestsPerMin   int
}

type rateLimitEntry struct {
	Count   int
	ResetAt time.Time
}

type ipRateLimiter struct {
	mu          sync.Mutex
	settings    rateLimitSettings
	entries     map[string]rateLimitEntry
	lastCleanup time.Time
}

func newIPRateLimiter(settings rateLimitSettings) *ipRateLimiter {
	return &ipRateLimiter{
		settings: settings,
		entries:  make(map[string]rateLimitEntry),
	}
}

func (l *ipRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		scope, limit := rateLimitScope(c.Request.URL.Path, l.settings)
		if limit <= 0 {
			c.Next()
			return
		}

		allowed, resetAt := l.allow(c.ClientIP()+"|"+scope, limit, time.Now().UTC())
		if !allowed {
			retryAfter := int(time.Until(resetAt).Seconds()) + 1
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests. Try again later."})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (l *ipRateLimiter) allow(key string, limit int, now time.Time) (bool, time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) > l.settings.Window {
		for entryKey, entry := range l.entries {
			if !entry.ResetAt.After(now) {
				delete(l.entries, entryKey)
			}
		}
		l.lastCleanup = now
	}

	entry := l.entries[key]
	if entry.ResetAt.IsZero() || !entry.ResetAt.After(now) {
		entry = rateLimitEntry{
			Count:   0,
			ResetAt: now.Add(l.settings.Window),
		}
	}

	entry.Count += 1
	l.entries[key] = entry

	return entry.Count <= limit, entry.ResetAt
}

func rateLimitScope(path string, settings rateLimitSettings) (string, int) {
	switch {
	case path == "/api/login" || path == "/api/register" || path == "/api/verify-email" || path == "/api/resend-verification":
		return "auth", settings.AuthRequestsPerMin
	case path == "/ws":
		return "ws", settings.WSRequestsPerMin
	case path == "/api" || strings.HasPrefix(path, "/api/"):
		return "api", settings.APIRequestsPerMin
	default:
		return "", 0
	}
}

func rateLimitSettingsFromEnv() rateLimitSettings {
	return rateLimitSettings{
		Window:             time.Minute,
		APIRequestsPerMin:  intEnv("API_RATE_LIMIT_REQUESTS_PER_MINUTE", 240),
		AuthRequestsPerMin: intEnv("AUTH_RATE_LIMIT_REQUESTS_PER_MINUTE", 20),
		WSRequestsPerMin:   intEnv("WS_RATE_LIMIT_REQUESTS_PER_MINUTE", 60),
	}
}

func intEnv(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		log.Printf("Invalid %s=%q, using %d", name, raw, fallback)
		return fallback
	}

	return value
}

func main() {
	// 1. Конфигурация
	loadDotEnv()

	dsn := os.Getenv("DB_DSN")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Println("Warning: JWT_SECRET is shorter than 32 characters; use a long random secret outside local development")
	}

	// 2. Инициализация инфраструктуры
	db := initDB(dsn)
	registry := initGameRegistry()

	// 3. Слой репозиториев (работа с БД)
	userRepo := users.NewRepository(db)
	gameRepo := game.NewRepository(db)

	// 4. Фоновые процессы (горутины)
	matchmaker := matchmaking.NewMatchmaker(registry, gameRepo, userRepo)
	go matchmaker.Run()

	hub := ws.NewHub()
	go hub.Run()

	// 5. Бизнес-логика (сервисы и хендлеры)
	userService := users.NewServiceWithEmailSender(userRepo, jwtSecret, users.NewEmailSenderFromEnv())
	userHandler := users.NewHandler(userService)

	// 6. Роутинг и запуск сервера
	router := SetupRouter(userHandler, hub, userRepo, jwtSecret, matchmaker, gameRepo)

	log.Println("Starting server on port " + port)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Error starting server: %v", err)
	}
}

func loadDotEnv() {
	for _, path := range envFileCandidates() {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}

	log.Println("Warning: Error loading .env file (fallback to system env)")
}

func envFileCandidates() []string {
	candidates := []string{
		"configs/.env",
		"chess-monolith/configs/.env",
	}

	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "configs", ".env"))
	}

	return candidates
}
