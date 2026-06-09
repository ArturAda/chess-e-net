package ws

import (
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/session"
	"errors"
	"fmt"
	"log"
	"time"
)

func (c *Client) handleDrawOffer() {
	if !c.isInActiveGameWithOpponent() {
		c.SendError(ErrorCodeNotInGame, "You are not in an active game", true)
		return
	}

	now := time.Now()
	offer, err := c.ActiveGame.CreateDrawOffer(c.Color, c.UserID, now)
	if err != nil {
		if errors.Is(err, session.ErrDrawOfferAlreadyActive) {
			c.SendError(ErrorCodeDrawOfferActive, "A draw offer is already active", true)
			return
		}
		c.SendError(ErrorCodeDrawOfferState, err.Error(), true)
		return
	}

	payload := buildDrawOfferPayload(offer, now)
	c.sendToPlayers(MessageTypeDrawOffer, payload)
	c.scheduleDrawOfferExpiration(offer)
	log.Printf("Player %s offered a draw.", c.UserID)
}

func (c *Client) handleDrawAccept() {
	if !c.isInActiveGameWithOpponent() {
		c.SendError(ErrorCodeNotInGame, "You are not in an active game", true)
		return
	}

	offer, err := c.ActiveGame.AcceptDrawOffer(c.Color, time.Now())
	if err != nil {
		c.handleDrawResponseError(offer, err)
		return
	}

	payload := buildDrawOfferResultPayload(offer, c.Color, c.UserID, "Draw offer accepted.")
	c.sendToPlayers(MessageTypeDrawAccepted, payload)
	log.Printf("Player %s accepted the draw.", c.UserID)
	c.ActiveGame.EndGame("draw")
}

func (c *Client) handleDrawDecline() {
	if !c.isInActiveGameWithOpponent() {
		c.SendError(ErrorCodeNotInGame, "You are not in an active game", true)
		return
	}

	offer, err := c.ActiveGame.DeclineDrawOffer(c.Color, time.Now())
	if err != nil {
		c.handleDrawResponseError(offer, err)
		return
	}

	payload := buildDrawOfferResultPayload(offer, c.Color, c.UserID, "Draw offer declined.")
	c.sendToPlayers(MessageTypeDrawDecline, payload)
	log.Printf("Player %s declined the draw.", c.UserID)
}

func (c *Client) handleDrawResponseError(offer session.DrawOfferState, err error) {
	if errors.Is(err, session.ErrDrawOfferExpired) {
		if offer.ID != "" {
			c.sendDrawExpired(offer)
		}
		c.SendError(ErrorCodeDrawOfferState, "Draw offer expired", true)
		return
	}
	if errors.Is(err, session.ErrDrawOfferOwnResponse) {
		c.SendError(ErrorCodeDrawOfferState, "You cannot respond to your own draw offer", true)
		return
	}
	if errors.Is(err, session.ErrDrawOfferNotFound) {
		c.SendError(ErrorCodeDrawOfferState, "No active draw offer", true)
		return
	}
	c.SendError(ErrorCodeDrawOfferState, err.Error(), true)
}

func (c *Client) scheduleDrawOfferExpiration(offer session.DrawOfferState) {
	delay := time.Until(offer.ExpiresAt)
	if delay < 0 {
		delay = 0
	}

	time.AfterFunc(delay, func() {
		if c == nil || c.ActiveGame == nil {
			return
		}

		expiredOffer, ok := c.ActiveGame.ExpireDrawOffer(offer.ID, time.Now())
		if !ok {
			return
		}
		c.sendDrawExpired(expiredOffer)
	})
}

func (c *Client) sendDrawExpired(offer session.DrawOfferState) {
	payload := buildDrawOfferResultPayload(offer, "", "", "Draw offer expired.")
	c.sendToPlayers(MessageTypeDrawExpired, payload)
	log.Printf("Draw offer %s expired.", offer.ID)
}

func (c *Client) sendToPlayers(messageType string, payload any) {
	c.SendMessage(messageType, payload)
	if c.Opponent != nil {
		c.Opponent.SendMessage(messageType, payload)
	}
}

func (c *Client) isInActiveGameWithOpponent() bool {
	if c == nil || c.ActiveGame == nil || c.Opponent == nil {
		return false
	}
	if c.Opponent.ActiveGame != c.ActiveGame || c.Opponent.Opponent != c {
		return false
	}

	c.ActiveGame.Mu.Lock()
	status := c.ActiveGame.Status
	c.ActiveGame.Mu.Unlock()
	return status == "active"
}

func buildDrawOfferPayload(offer session.DrawOfferState, now time.Time) DrawOfferPayload {
	expiresIn := offer.ExpiresAt.Sub(now).Milliseconds()
	if expiresIn < 0 {
		expiresIn = 0
	}

	return DrawOfferPayload{
		OfferID:         offer.ID,
		OfferedBy:       string(offer.OfferedBy),
		OfferedByUserID: offer.OfferedByUserID,
		ExpiresAt:       offer.ExpiresAt,
		ExpiresInMs:     expiresIn,
		Message:         fmt.Sprintf("%s offered a draw.", offer.OfferedBy),
	}
}

func buildDrawOfferResultPayload(offer session.DrawOfferState, respondedBy core.Color, respondedByUserID, message string) DrawOfferResultPayload {
	return DrawOfferResultPayload{
		OfferID:           offer.ID,
		OfferedBy:         string(offer.OfferedBy),
		OfferedByUserID:   offer.OfferedByUserID,
		RespondedBy:       string(respondedBy),
		RespondedByUserID: respondedByUserID,
		Message:           message,
	}
}
