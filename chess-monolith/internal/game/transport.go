package game

import (
	"chess-monolith/internal/users"
	"chess-monolith/pkg/jwtutil"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	repo      Repository
	userRepo  users.Repository
	jwtSecret string
}

type PlayerDTO struct {
	ID       string `json:"id"`
	Username string `json:"username,omitempty"`
	Rating   int    `json:"rating"`
}

type GameSummaryDTO struct {
	ID          string          `json:"id"`
	Mode        string          `json:"mode"`
	BoardSize   int             `json:"board_size"`
	TimeLimitMs int64           `json:"time_limit_ms"`
	IsRanked    bool            `json:"is_ranked"`
	Status      string          `json:"status"`
	Turn        string          `json:"turn"`
	PlayerColor string          `json:"player_color"`
	Result      string          `json:"result"`
	White       PlayerDTO       `json:"white"`
	Black       PlayerDTO       `json:"black"`
	Opponent    PlayerDTO       `json:"opponent"`
	WinnerID    *string         `json:"winner_id,omitempty"`
	BoardState  json.RawMessage `json:"board_state"`
	VisualState json.RawMessage `json:"visual_state"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type GameDetailDTO struct {
	GameSummaryDTO
}

func NewHandler(repo Repository, userRepo users.Repository, jwtSecret string) *Handler {
	return &Handler{
		repo:      repo,
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

func (h *Handler) SetupRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		api.GET("/games", h.ListGames)
		api.GET("/games/:id", h.GetGame)
	}
}

func (h *Handler) ListGames(c *gin.Context) {
	userID, ok := h.authorize(c)
	if !ok {
		return
	}

	games, err := h.repo.ListGamesForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load games"})
		return
	}

	response := make([]GameSummaryDTO, 0, len(games))
	for _, item := range games {
		response = append(response, h.buildSummaryDTO(item, userID))
	}

	c.JSON(http.StatusOK, gin.H{"games": response})
}

func (h *Handler) GetGame(c *gin.Context) {
	userID, ok := h.authorize(c)
	if !ok {
		return
	}

	gameID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid game id"})
		return
	}

	item, err := h.repo.GetGameForUser(gameID, userID)
	if err != nil {
		if errors.Is(err, ErrGameNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Game not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load game"})
		return
	}

	c.JSON(http.StatusOK, h.buildDetailDTO(*item, userID))
}

func (h *Handler) authorize(c *gin.Context) (uuid.UUID, bool) {
	token, ok := bearerTokenFromHeader(c.GetHeader("Authorization"))
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization bearer token is required"})
		return uuid.Nil, false
	}

	userID, err := jwtutil.ParseToken(token, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return uuid.Nil, false
	}

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return uuid.Nil, false
	}

	if _, err := h.userRepo.GetUserByID(parsedUserID); err != nil {
		if errors.Is(err, users.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return uuid.Nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to authorize user"})
		return uuid.Nil, false
	}

	return parsedUserID, true
}

func (h *Handler) buildDetailDTO(item Game, currentUserID uuid.UUID) GameDetailDTO {
	return GameDetailDTO{
		GameSummaryDTO: h.buildSummaryDTO(item, currentUserID),
	}
}

func (h *Handler) buildSummaryDTO(item Game, currentUserID uuid.UUID) GameSummaryDTO {
	white := h.buildPlayerDTO(item.WhiteID)
	black := h.buildPlayerDTO(item.BlackID)
	opponent := black
	if item.BlackID == currentUserID {
		opponent = white
	}

	return GameSummaryDTO{
		ID:          item.ID.String(),
		Mode:        item.Mode,
		BoardSize:   item.BoardSize,
		TimeLimitMs: item.TimeLimitMs,
		IsRanked:    item.IsRanked,
		Status:      item.Status,
		Turn:        item.Turn,
		PlayerColor: playerColorForGame(item, currentUserID),
		Result:      resultForUser(item, currentUserID),
		White:       white,
		Black:       black,
		Opponent:    opponent,
		WinnerID:    winnerIDString(item.WinnerID),
		BoardState:  normalizedJSONState(item.BoardState),
		VisualState: visualStateForUser(item, currentUserID),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func (h *Handler) buildPlayerDTO(userID uuid.UUID) PlayerDTO {
	player := PlayerDTO{ID: userID.String()}
	user, err := h.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return player
	}
	player.Username = user.Username
	player.Rating = user.Rating
	return player
}

func bearerTokenFromHeader(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], true
}

func normalizedJSONState(value string) json.RawMessage {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(trimmed)
}

func visualStateForUser(item Game, userID uuid.UUID) json.RawMessage {
	if item.WhiteID == userID {
		return normalizedJSONState(item.WhiteVisualState)
	}
	if item.BlackID == userID {
		return normalizedJSONState(item.BlackVisualState)
	}
	return json.RawMessage(`{}`)
}

func playerColorForGame(item Game, userID uuid.UUID) string {
	if item.WhiteID == userID {
		return "white"
	}
	if item.BlackID == userID {
		return "black"
	}
	return ""
}

func resultForUser(item Game, userID uuid.UUID) string {
	if item.Status == "active" {
		return "active"
	}
	if item.Status == StaleActiveGameStatus {
		return StaleActiveGameStatus
	}
	if strings.Contains(item.Status, "draw") {
		return "draw"
	}

	whiteWon := strings.HasPrefix(item.Status, "white_won")
	blackWon := strings.HasPrefix(item.Status, "black_won")
	if !whiteWon && !blackWon {
		return "unknown"
	}

	userIsWhite := item.WhiteID == userID
	if whiteWon == userIsWhite {
		return "win"
	}
	return "loss"
}

func winnerIDString(winnerID *uuid.UUID) *string {
	if winnerID == nil {
		return nil
	}
	value := winnerID.String()
	return &value
}
