package ws

import (
	"fmt"
	"sync"
	"time"
)

const (
	NetworkIdleThreshold = 15 * time.Second
	NetworkLossGrace     = time.Minute
	networkMonitorPeriod = time.Second
	networkHeartbeatPing = 5 * time.Second
)

type networkActivityState struct {
	mu        sync.Mutex
	lastSeen  time.Time
	waiting   bool
	expiresAt time.Time
}

func (c *Client) markNetworkActivity(now time.Time) {
	if c == nil {
		return
	}

	c.networkActivity.mu.Lock()
	wasWaiting := c.networkActivity.waiting
	c.networkActivity.lastSeen = now
	c.networkActivity.waiting = false
	c.networkActivity.expiresAt = time.Time{}
	c.networkActivity.mu.Unlock()

	if wasWaiting && c.isInActiveGameWithOpponent() {
		c.sendToPlayers(MessageTypePlayerNetworkRestored, c.buildPlayerNetworkRestoredPayload())
	}
}

func (c *Client) monitorNetworkActivity(done <-chan struct{}) {
	ticker := time.NewTicker(networkMonitorPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			c.checkNetworkActivity(now)
		}
	}
}

func (c *Client) checkNetworkActivity(now time.Time) {
	if c == nil {
		return
	}

	if !c.isInActiveGameWithOpponent() {
		c.clearNetworkWaiting()
		return
	}

	c.networkActivity.mu.Lock()
	if c.networkActivity.lastSeen.IsZero() {
		c.networkActivity.lastSeen = now
		c.networkActivity.mu.Unlock()
		return
	}

	if c.networkActivity.waiting {
		expiresAt := c.networkActivity.expiresAt
		if now.Before(expiresAt) {
			c.networkActivity.mu.Unlock()
			return
		}

		c.networkActivity.waiting = false
		c.networkActivity.expiresAt = time.Time{}
		c.networkActivity.mu.Unlock()

		c.handleNetworkTimeoutLoss()
		return
	}

	if now.Sub(c.networkActivity.lastSeen) < NetworkIdleThreshold {
		c.networkActivity.mu.Unlock()
		return
	}

	expiresAt := now.Add(NetworkLossGrace)
	c.networkActivity.waiting = true
	c.networkActivity.expiresAt = expiresAt
	c.networkActivity.mu.Unlock()

	c.sendToPlayers(MessageTypePlayerNetworkWaiting, c.buildPlayerNetworkWaitingPayload(now, expiresAt))
}

func (c *Client) clearNetworkWaiting() {
	if c == nil {
		return
	}

	c.networkActivity.mu.Lock()
	c.networkActivity.waiting = false
	c.networkActivity.expiresAt = time.Time{}
	c.networkActivity.mu.Unlock()
}

func (c *Client) buildPlayerNetworkWaitingPayload(now, expiresAt time.Time) PlayerNetworkWaitingPayload {
	remainingMs := expiresAt.Sub(now).Milliseconds()
	if remainingMs < 0 {
		remainingMs = 0
	}

	return PlayerNetworkWaitingPayload{
		UserID:      c.UserID,
		Username:    c.Username,
		Color:       string(c.Color),
		ExpiresAt:   expiresAt,
		RemainingMs: remainingMs,
		Message:     fmt.Sprintf("Waiting for %s network.", c.UsernameOrFallback()),
	}
}

func (c *Client) buildPlayerNetworkRestoredPayload() PlayerNetworkRestoredPayload {
	return PlayerNetworkRestoredPayload{
		UserID:   c.UserID,
		Username: c.Username,
		Color:    string(c.Color),
		Message:  fmt.Sprintf("%s network restored.", c.UsernameOrFallback()),
	}
}

func (c *Client) UsernameOrFallback() string {
	if c == nil {
		return "player"
	}
	if c.Username != "" {
		return c.Username
	}
	if c.UserID != "" {
		return fmt.Sprintf("Player %s", c.UserID)
	}
	return "player"
}
