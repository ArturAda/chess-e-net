package users

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) SetupRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		api.POST("/register", h.Register)
		api.POST("/login", h.Login)
		api.GET("/me", h.Me)
		api.GET("/me/ratings", h.MeRatings)
		api.GET("/leaderboard", h.Leaderboard)
	}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data: " + err.Error()})
		return
	}

	if err := h.service.Register(req.Username, req.Email, req.Password); err != nil {
		if errors.Is(err, ErrUserExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
			return
		}

		if errors.Is(err, ErrDatabase) {
			log.Printf("[ERROR] Database failure during registration: %v", err)
		} else {
			log.Printf("[ERROR] Unknown failure during registration: %v", err)
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data: " + err.Error()})
		return
	}

	token, err := h.service.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}

		if errors.Is(err, ErrDatabase) {
			log.Printf("[ERROR] Database failure during login: %v", err)
		} else {
			log.Printf("[ERROR] Unknown failure during login: %v", err)
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"message": "Login successful",
	})
}

func (h *Handler) Me(c *gin.Context) {
	token, ok := bearerTokenFromHeader(c.GetHeader("Authorization"))
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization bearer token is required"})
		return
	}

	profile, err := h.service.GetCurrentUser(token)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		if errors.Is(err, ErrDatabase) {
			log.Printf("[ERROR] Database failure during current user lookup: %v", err)
		} else {
			log.Printf("[ERROR] Unknown failure during current user lookup: %v", err)
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

func (h *Handler) MeRatings(c *gin.Context) {
	token, ok := bearerTokenFromHeader(c.GetHeader("Authorization"))
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization bearer token is required"})
		return
	}

	ratings, err := h.service.GetCurrentUserRatings(token)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		if errors.Is(err, ErrDatabase) {
			log.Printf("[ERROR] Database failure during current user ratings lookup: %v", err)
		} else {
			log.Printf("[ERROR] Unknown failure during current user ratings lookup: %v", err)
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ratings": ratings})
}

func (h *Handler) Leaderboard(c *gin.Context) {
	scope, err := ratingScopeFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	limit, err := leaderboardLimitFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	leaderboard, err := h.service.GetLeaderboard(scope, limit)
	if err != nil {
		if errors.Is(err, ErrDatabase) {
			log.Printf("[ERROR] Database failure during leaderboard lookup: %v", err)
		} else {
			log.Printf("[ERROR] Unknown failure during leaderboard lookup: %v", err)
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, leaderboard)
}

func bearerTokenFromHeader(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
}

func ratingScopeFromQuery(c *gin.Context) (RatingScope, error) {
	mode := strings.TrimSpace(c.DefaultQuery("mode", defaultRatingMode))
	boardSize, err := intQuery(c, "board_size", defaultRatingBoardSize)
	if err != nil {
		return RatingScope{}, err
	}

	timeLimitMs, err := timeLimitMsFromQuery(c)
	if err != nil {
		return RatingScope{}, err
	}

	scope := normalizeRatingScope(RatingScope{
		Mode:        mode,
		BoardSize:   boardSize,
		TimeLimitMs: timeLimitMs,
	})
	if !isSupportedRatingScope(scope) {
		return RatingScope{}, errors.New("unsupported rating scope")
	}

	return scope, nil
}

func leaderboardLimitFromQuery(c *gin.Context) (int, error) {
	limit, err := intQuery(c, "limit", 50)
	if err != nil {
		return 0, err
	}
	if limit <= 0 {
		return 0, errors.New("limit must be positive")
	}

	return normalizeLeaderboardLimit(limit), nil
}

func timeLimitMsFromQuery(c *gin.Context) (int64, error) {
	if raw := strings.TrimSpace(c.Query("time_limit_ms")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return 0, errors.New("time_limit_ms must be a positive integer")
		}

		return value, nil
	}

	if raw := strings.TrimSpace(c.Query("time_limit_minutes")); raw != "" {
		minutes, err := strconv.Atoi(raw)
		if err != nil || minutes <= 0 {
			return 0, errors.New("time_limit_minutes must be a positive integer")
		}

		return minutesToMilliseconds(minutes), nil
	}

	if raw := strings.TrimSpace(c.Query("time_limit")); raw != "" {
		minutes, err := strconv.Atoi(raw)
		if err != nil || minutes <= 0 {
			return 0, errors.New("time_limit must be a positive integer")
		}

		return minutesToMilliseconds(minutes), nil
	}

	return defaultRatingTimeLimitMs, nil
}

func intQuery(c *gin.Context, name string, fallback int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New(name + " must be an integer")
	}

	return value, nil
}
