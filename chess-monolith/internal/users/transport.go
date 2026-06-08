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
	Username       string `json:"username" binding:"required,min=3,max=50"`
	Email          string `json:"email" binding:"required,email,max=100"`
	Password       string `json:"password" binding:"required,min=6,max=72"`
	TurnstileToken string `json:"turnstile_token"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=100"`
	Password string `json:"password" binding:"required,min=6,max=72"`
}

type VerifyEmailRequest struct {
	Email string `json:"email" binding:"required,email,max=100"`
	Code  string `json:"code" binding:"required,len=6,numeric"`
}

type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email,max=100"`
}

type Handler struct {
	service         Service
	captchaVerifier CaptchaVerifier
}

func NewHandler(service Service) *Handler {
	return NewHandlerWithCaptcha(service, NewTurnstileVerifierFromEnv())
}

func NewHandlerWithCaptcha(service Service, captchaVerifier CaptchaVerifier) *Handler {
	return &Handler{
		service:         service,
		captchaVerifier: captchaVerifier,
	}
}

func (h *Handler) SetupRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		api.GET("/config", h.Config)
		api.POST("/register", h.Register)
		api.POST("/login", h.Login)
		api.POST("/verify-email", h.VerifyEmail)
		api.POST("/resend-verification", h.ResendVerification)
		api.GET("/me", h.Me)
		api.GET("/me/ratings", h.MeRatings)
		api.GET("/leaderboard", h.Leaderboard)
	}
}

func (h *Handler) Config(c *gin.Context) {
	enabled := h.captchaVerifier != nil && h.captchaVerifier.Enabled()
	siteKey := ""
	if h.captchaVerifier != nil {
		siteKey = h.captchaVerifier.SiteKey()
	}

	c.JSON(http.StatusOK, gin.H{
		"turnstile": gin.H{
			"enabled":  enabled,
			"site_key": siteKey,
		},
	})
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data: " + err.Error()})
		return
	}

	if err := h.verifyCaptcha(c, req.TurnstileToken); err != nil {
		if errors.Is(err, ErrCaptchaRequired) || errors.Is(err, ErrCaptchaInvalid) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Human verification failed"})
			return
		}

		log.Printf("[ERROR] Captcha verification failure during registration: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Human verification is unavailable"})
		return
	}

	if err := h.service.Register(req.Username, req.Email, req.Password); err != nil {
		if errors.Is(err, ErrUserExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
			return
		}
		if errors.Is(err, ErrEmailDelivery) {
			log.Printf("[ERROR] Email delivery failure during registration: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Verification email could not be sent"})
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

	c.JSON(http.StatusCreated, gin.H{"message": "User created. Check your email for the verification code."})
}

func (h *Handler) verifyCaptcha(c *gin.Context, token string) error {
	if h.captchaVerifier == nil || !h.captchaVerifier.Enabled() {
		return nil
	}

	return h.captchaVerifier.Verify(c.Request.Context(), token, c.ClientIP())
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
		if errors.Is(err, ErrEmailNotVerified) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Email is not verified. New verification code sent."})
			return
		}
		if errors.Is(err, ErrEmailDelivery) {
			log.Printf("[ERROR] Email delivery failure during login verification resend: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Verification email could not be sent"})
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

func (h *Handler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data: " + err.Error()})
		return
	}

	if err := h.service.VerifyEmail(req.Email, req.Code); err != nil {
		if errors.Is(err, ErrInvalidCode) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid verification code"})
			return
		}
		if errors.Is(err, ErrCodeExpired) {
			c.JSON(http.StatusGone, gin.H{"error": "Verification code expired"})
			return
		}
		if errors.Is(err, ErrDatabase) {
			log.Printf("[ERROR] Database failure during email verification: %v", err)
		} else {
			log.Printf("[ERROR] Unknown failure during email verification: %v", err)
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email verified"})
}

func (h *Handler) ResendVerification(c *gin.Context) {
	var req ResendVerificationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data: " + err.Error()})
		return
	}

	if err := h.service.ResendVerificationCode(req.Email); err != nil {
		if errors.Is(err, ErrEmailDelivery) {
			log.Printf("[ERROR] Email delivery failure during verification resend: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Verification email could not be sent"})
			return
		}
		if errors.Is(err, ErrDatabase) {
			log.Printf("[ERROR] Database failure during verification resend: %v", err)
		} else {
			log.Printf("[ERROR] Unknown failure during verification resend: %v", err)
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If this email is registered and not verified, a new code was sent."})
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
