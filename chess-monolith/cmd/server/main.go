package main

import (
	"errors"
	"html"
	"log"
	"net"
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

func runDocumentationServer(addr string) error {
	router := gin.Default()
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Printf("Unable to disable trusted proxies: %v", err)
	}

	router.Use(securityHeaders())

	router.GET("/", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", renderDocumentationPage("README.ru.md"))
	})
	router.GET("/README.md", func(c *gin.Context) {
		serveDocumentationFile(c, "README.md")
	})
	router.GET("/README.ru.md", func(c *gin.Context) {
		serveDocumentationFile(c, "README.ru.md")
	})
	router.GET("/LICENSE", func(c *gin.Context) {
		serveDocumentationFile(c, "LICENSE")
	})
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	})

	log.Println("Documentation server starting on " + addr)
	return router.Run(addr)
}

func serveDocumentationFile(c *gin.Context, name string) {
	content, _, err := readDocumentationFile(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}

	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", renderDocumentationPage(name, content))
}

func renderDocumentationPage(name string, content ...[]byte) []byte {
	body := []byte(nil)
	if len(content) > 0 {
		body = content[0]
	} else if fileContent, _, err := readDocumentationFile(name); err == nil {
		body = fileContent
	} else if name != "README.md" {
		name = "README.md"
		fileContent, _, err := readDocumentationFile(name)
		if err == nil {
			body = fileContent
		}
	}
	if body == nil {
		return renderDocumentationDocument("Chess-E-Net documentation", "README.ru.md", []byte("Documentation files not found"))
	}

	return renderDocumentationDocument(documentationTitle(name), name, body)
}

func renderDocumentationDocument(title string, name string, content []byte) []byte {
	escapedTitle := html.EscapeString(title)
	bodyHTML := renderDocumentationBody(name, content)

	return []byte(`<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + escapedTitle + `</title>
    <style>
        :root {
            color-scheme: light;
            --ink: #17211b;
            --muted: #68726c;
            --paper: #fffdf6;
            --line: #d9dfd2;
            --accent: #2f6f62;
            --accent-strong: #184b43;
            --code-bg: #17211b;
        }
        * { box-sizing: border-box; }
        body {
            margin: 0;
            min-height: 100vh;
            padding: 40px 20px;
            font-family: Georgia, "Times New Roman", serif;
            background:
                linear-gradient(90deg, rgba(23, 33, 27, 0.06) 1px, transparent 1px),
                linear-gradient(180deg, rgba(23, 33, 27, 0.05) 1px, transparent 1px),
                #ecefe4;
            background-size: 24px 24px;
            color: var(--ink);
        }
        .wrap {
            max-width: 980px;
            margin: 0 auto;
            padding: 34px;
            background: var(--paper);
            border: 1px solid var(--line);
            box-shadow: 0 24px 70px rgba(31, 45, 39, 0.16);
        }
        .topbar {
            display: flex;
            gap: 10px;
            flex-wrap: wrap;
            align-items: center;
            justify-content: space-between;
            padding-bottom: 18px;
            margin-bottom: 26px;
            border-bottom: 1px solid var(--line);
            font-family: "Trebuchet MS", Verdana, sans-serif;
        }
        .brand { font-weight: 900; letter-spacing: 0; color: var(--accent-strong); }
        .links { display: flex; gap: 8px; flex-wrap: wrap; }
        .topbar a {
            display: inline-flex;
            align-items: center;
            min-height: 34px;
            padding: 0 12px;
            border: 1px solid var(--line);
            color: var(--accent-strong);
            text-decoration: none;
            font-weight: 800;
            background: #f7f5ec;
            border-radius: 6px;
        }
        .doc {
            font-size: 18px;
            line-height: 1.68;
        }
        .doc h1, .doc h2, .doc h3, .doc h4 {
            font-family: "Trebuchet MS", Verdana, sans-serif;
            line-height: 1.15;
            margin: 1.45em 0 0.5em;
            color: var(--accent-strong);
            letter-spacing: 0;
        }
        .doc h1 {
            margin-top: 0;
            font-size: 42px;
        }
        .doc h2 {
            padding-top: 18px;
            border-top: 1px solid var(--line);
            font-size: 29px;
        }
        .doc h3 { font-size: 23px; }
        .doc h4 { font-size: 19px; }
        .doc p { margin: 0 0 1em; }
        .doc a { color: var(--accent); font-weight: 700; }
        .doc ul, .doc ol {
            margin: 0 0 1.15em;
            padding-left: 1.35em;
        }
        .doc li { margin: 0.35em 0; }
        .doc blockquote {
            margin: 1.2em 0;
            padding: 0.4em 1em;
            border-left: 4px solid var(--accent);
            background: #f3f5ec;
            color: #344238;
        }
        .doc code {
            padding: 0.12em 0.35em;
            border-radius: 4px;
            background: #e9ede2;
            font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
            font-size: 0.9em;
        }
        pre {
            margin: 1.2em 0;
            padding: 18px;
            overflow-x: auto;
            white-space: pre-wrap;
            overflow-wrap: anywhere;
            word-break: break-word;
            background: var(--code-bg);
            color: #f5f3e8;
            border-radius: 8px;
        }
        .doc pre code {
            display: block;
            padding: 0;
            background: transparent;
            color: inherit;
            border-radius: 0;
            font-size: inherit;
        }
        .license-text p {
            margin-bottom: 1.1em;
        }
        @media (max-width: 720px) {
            body { padding: 14px; }
            .wrap { padding: 22px; }
            .doc { font-size: 16px; }
            .doc h1 { font-size: 31px; }
        }
    </style>
</head>
<body>
    <div class="wrap">
        <div class="topbar">
            <strong class="brand">Chess-E-Net docs</strong>
	            <nav class="links" aria-label="Documentation">
	                <a href="/">Главная</a>
	                <a href="/README.ru.md">README.ru.md</a>
	                <a href="/README.md">README.md</a>
	                <a href="/LICENSE">LICENSE</a>
	            </nav>
        </div>
        <main class="doc">` + bodyHTML + `</main>
    </div>
</body>
</html>`)
}

func documentationTitle(name string) string {
	switch name {
	case "README.md":
		return "Chess-E-Net documentation"
	case "README.ru.md":
		return "Документация Chess-E-Net"
	case "LICENSE":
		return "MIT License"
	default:
		return "Chess-E-Net documentation"
	}
}

func renderDocumentationBody(name string, content []byte) string {
	if name == "README.md" || name == "README.ru.md" {
		return renderMarkdownHTML(string(content))
	}
	return renderPlainTextHTML(string(content))
}

func renderPlainTextHTML(content string) string {
	paragraphs := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n\n")
	var b strings.Builder
	b.WriteString(`<section class="license-text">`)
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		b.WriteString("<p>")
		b.WriteString(strings.ReplaceAll(html.EscapeString(paragraph), "\n", "<br>"))
		b.WriteString("</p>")
	}
	b.WriteString("</section>")
	return b.String()
}

func renderMarkdownHTML(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var b strings.Builder
	var paragraph []string
	listTag := ""
	inCodeBlock := false
	inComment := false

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		b.WriteString("<p>")
		b.WriteString(renderInlineMarkdown(strings.Join(paragraph, " ")))
		b.WriteString("</p>")
		paragraph = nil
	}
	closeList := func() {
		if listTag == "" {
			return
		}
		b.WriteString("</")
		b.WriteString(listTag)
		b.WriteString(">")
		listTag = ""
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inComment {
			if strings.Contains(trimmed, "-->") {
				inComment = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "<!--") {
			inComment = !strings.Contains(trimmed, "-->")
			continue
		}
		if strings.HasPrefix(trimmed, "[![") {
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			flushParagraph()
			closeList()
			if inCodeBlock {
				b.WriteString("</code></pre>")
				inCodeBlock = false
			} else {
				b.WriteString("<pre><code>")
				inCodeBlock = true
			}
			continue
		}
		if inCodeBlock {
			b.WriteString(html.EscapeString(line))
			b.WriteByte('\n')
			continue
		}
		if trimmed == "" {
			flushParagraph()
			closeList()
			continue
		}
		if level, text := markdownHeading(trimmed); level > 0 {
			flushParagraph()
			closeList()
			b.WriteString("<h")
			b.WriteString(strconv.Itoa(level))
			b.WriteString(">")
			b.WriteString(renderInlineMarkdown(text))
			b.WriteString("</h")
			b.WriteString(strconv.Itoa(level))
			b.WriteString(">")
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			flushParagraph()
			closeList()
			b.WriteString("<blockquote>")
			b.WriteString(renderInlineMarkdown(strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))))
			b.WriteString("</blockquote>")
			continue
		}
		if item, ok := unorderedListItem(trimmed); ok {
			flushParagraph()
			if listTag != "ul" {
				closeList()
				b.WriteString("<ul>")
				listTag = "ul"
			}
			b.WriteString("<li>")
			b.WriteString(renderInlineMarkdown(item))
			b.WriteString("</li>")
			continue
		}
		if item, ok := orderedListItem(trimmed); ok {
			flushParagraph()
			if listTag != "ol" {
				closeList()
				b.WriteString("<ol>")
				listTag = "ol"
			}
			b.WriteString("<li>")
			b.WriteString(renderInlineMarkdown(item))
			b.WriteString("</li>")
			continue
		}

		closeList()
		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	closeList()
	if inCodeBlock {
		b.WriteString("</code></pre>")
	}
	return b.String()
}

func markdownHeading(line string) (int, string) {
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, ""
	}
	return level, strings.TrimSpace(line[level+1:])
}

func unorderedListItem(line string) (string, bool) {
	for _, marker := range []string{"* ", "- "} {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(strings.TrimPrefix(line, marker)), true
		}
	}
	return "", false
}

func orderedListItem(line string) (string, bool) {
	index := 0
	for index < len(line) && line[index] >= '0' && line[index] <= '9' {
		index++
	}
	if index == 0 || index+1 >= len(line) || line[index] != '.' || line[index+1] != ' ' {
		return "", false
	}
	return strings.TrimSpace(line[index+2:]), true
}

func renderInlineMarkdown(text string) string {
	escaped := html.EscapeString(text)
	escaped = replaceMarkdownLinks(escaped)
	escaped = replaceInlineCode(escaped)
	escaped = replaceStrongText(escaped)
	return escaped
}

func replaceMarkdownLinks(text string) string {
	var b strings.Builder
	for {
		closeText := strings.Index(text, "](")
		if closeText == -1 {
			b.WriteString(text)
			break
		}
		openText := strings.LastIndex(text[:closeText], "[")
		closeURL := strings.Index(text[closeText+2:], ")")
		if openText == -1 || closeURL == -1 {
			b.WriteString(text)
			break
		}

		closeURL += closeText + 2
		label := text[openText+1 : closeText]
		href := html.UnescapeString(text[closeText+2 : closeURL])
		b.WriteString(text[:openText])
		if isSafeDocumentationHref(href) {
			b.WriteString(`<a href="`)
			b.WriteString(html.EscapeString(href))
			b.WriteString(`">`)
			b.WriteString(label)
			b.WriteString(`</a>`)
		} else {
			b.WriteString(label)
		}
		text = text[closeURL+1:]
	}
	return b.String()
}

func replaceInlineCode(text string) string {
	parts := strings.Split(text, "`")
	if len(parts) < 3 {
		return text
	}
	var b strings.Builder
	for index, part := range parts {
		if index%2 == 1 {
			b.WriteString("<code>")
			b.WriteString(part)
			b.WriteString("</code>")
		} else {
			b.WriteString(part)
		}
	}
	return b.String()
}

func replaceStrongText(text string) string {
	for {
		start := strings.Index(text, "**")
		if start == -1 {
			return text
		}
		end := strings.Index(text[start+2:], "**")
		if end == -1 {
			return text
		}
		end += start + 2
		text = text[:start] + "<strong>" + text[start+2:end] + "</strong>" + text[end+2:]
	}
}

func isSafeDocumentationHref(href string) bool {
	href = strings.TrimSpace(href)
	lower := strings.ToLower(href)
	if href == "" || strings.ContainsRune(href, 0) || strings.HasPrefix(href, "//") || strings.Contains(href, "..") {
		return false
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") {
		return true
	}
	return !strings.Contains(href, ":")
}

func readDocumentationFile(name string) ([]byte, string, error) {
	name = strings.TrimSpace(name)
	if !isAllowedDocumentationFile(name) {
		return nil, "", os.ErrInvalid
	}

	for _, candidate := range documentationFileCandidates(name) {
		content, err := os.ReadFile(candidate)
		if err == nil {
			return content, candidate, nil
		}
	}

	return nil, "", os.ErrNotExist
}

func isAllowedDocumentationFile(name string) bool {
	switch name {
	case "README.md", "README.ru.md", "LICENSE":
		return true
	default:
		return false
	}
}

func documentationFileCandidates(name string) []string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return nil
	}

	candidates := make([]string, 0, 6)
	for _, root := range documentationRootCandidates() {
		if isMonolithRoot(root) {
			candidates = appendUniquePath(candidates, filepath.Join(root, name))
			if name == "LICENSE" {
				candidates = appendUniquePath(candidates, filepath.Join(filepath.Dir(root), name))
			}
		}
		if isRepositoryRoot(root) {
			candidates = appendUniquePath(candidates, filepath.Join(root, name))
			candidates = appendUniquePath(candidates, filepath.Join(root, "chess-monolith", name))
		}
	}

	return candidates
}

func documentationRootCandidates() []string {
	candidates := make([]string, 0, 10)
	if currentDir, err := os.Getwd(); err == nil && currentDir != "" {
		candidates = appendDocumentationRootCandidates(candidates, currentDir)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = appendDocumentationRootCandidates(candidates, filepath.Dir(executable))
	}
	return candidates
}

func appendDocumentationRootCandidates(candidates []string, startDir string) []string {
	dir := filepath.Clean(startDir)
	for depth := 0; depth < 6; depth++ {
		candidates = appendUniquePath(candidates, dir)
		candidates = appendUniquePath(candidates, filepath.Join(dir, "chess-monolith"))

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return candidates
}

func isMonolithRoot(dir string) bool {
	return fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "cmd", "server", "main.go"))
}

func isRepositoryRoot(dir string) bool {
	return fileExists(filepath.Join(dir, "chess-monolith", "go.mod")) && fileExists(filepath.Join(dir, "README.md"))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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
	loadDotEnv()

	// Запускаем сервер документации в отдельной горутине
	docsAddr := documentationAddressFromEnv()
	go func() {
		if err := runDocumentationServer(docsAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Documentation server is not available on %s: %v", docsAddr, err)
		}
	}()

	// Основной игровой сервер
	gameAddr := gameAddressFromEnv()

	dsn := os.Getenv("DB_DSN")
	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Println("Warning: JWT_SECRET is shorter than 32 characters; use a long random secret outside local development")
	}

	db := initDB(dsn)
	registry := initGameRegistry()

	userRepo := users.NewRepository(db)
	gameRepo := game.NewRepository(db)

	matchmaker := matchmaking.NewMatchmaker(registry, gameRepo, userRepo)
	go matchmaker.Run()

	hub := ws.NewHub()
	go hub.Run()

	userService := users.NewServiceWithEmailSender(userRepo, jwtSecret, users.NewEmailSenderFromEnv())
	userHandler := users.NewHandler(userService)

	router := SetupRouter(userHandler, hub, userRepo, jwtSecret, matchmaker, gameRepo)

	log.Println("Starting game server on " + gameAddr)
	server := &http.Server{
		Addr:              gameAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Error starting game server: %v", err)
	}
}

func gameAddressFromEnv() string {
	host := strings.TrimSpace(os.Getenv("HOST"))
	if host == "" {
		host = "127.0.0.1"
	}
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	return net.JoinHostPort(host, port)
}

func documentationAddressFromEnv() string {
	host := strings.TrimSpace(os.Getenv("DOCS_HOST"))
	if host == "" {
		host = "127.0.0.1"
	}
	port := strings.TrimSpace(os.Getenv("DOCS_PORT"))
	if port == "" {
		port = "65000"
	}
	return net.JoinHostPort(host, port)
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
