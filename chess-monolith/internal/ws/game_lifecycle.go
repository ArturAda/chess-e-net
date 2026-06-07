package ws

import (
	"chess-monolith/internal/game/core"
	"log"
)

func (c *Client) handleResign() {
	status, ok := c.endActiveGameAsCurrentPlayerLoss()
	if ok {
		log.Printf("Player %s resigned. Final status: %s", c.UserID, status)
	}
}

func (c *Client) handleLeaveGame() {
	status, ok := c.endActiveGameAsCurrentPlayerLoss()
	if ok {
		log.Printf("Player %s left the game. Final status: %s", c.UserID, status)
	}
}

func (c *Client) handleDisconnectLoss() {
	status, ok := c.endActiveGameAsCurrentPlayerLoss()
	if ok {
		log.Printf("Player %s disconnected. Technical defeat applied. Final status: %s", c.UserID, status)
	}
}

func (c *Client) handleNetworkTimeoutLoss() {
	status, ok := c.endActiveGameAsCurrentPlayerLoss()
	if ok {
		log.Printf("Player %s lost by network timeout. Final status: %s", c.UserID, status)
	}
}

func (c *Client) endActiveGameAsCurrentPlayerLoss() (string, bool) {
	if c == nil || c.ActiveGame == nil {
		return "", false
	}

	c.ActiveGame.Mu.Lock()
	status := c.ActiveGame.Status
	c.ActiveGame.Mu.Unlock()

	if status != "active" {
		return "", false
	}

	finalStatus := winnerStatusAfterCurrentPlayerLoss(c.Color)
	c.ActiveGame.EndGame(finalStatus)
	return finalStatus, true
}

func winnerStatusAfterCurrentPlayerLoss(color core.Color) string {
	if color == core.White {
		return "black_won_resign"
	}
	return "white_won_resign"
}
