package session

import (
	"chess-monolith/internal/game/core"
	"errors"
	"time"

	"github.com/google/uuid"
)

const DrawOfferTTL = time.Minute

var (
	ErrDrawOfferAlreadyActive = errors.New("draw offer is already active")
	ErrDrawOfferNotFound      = errors.New("no active draw offer")
	ErrDrawOfferExpired       = errors.New("draw offer expired")
	ErrDrawOfferOwnResponse   = errors.New("player cannot respond to their own draw offer")
)

type DrawOfferState struct {
	ID              string
	OfferedBy       core.Color
	OfferedByUserID string
	ExpiresAt       time.Time
}

func (s *GameSession) CreateDrawOffer(offeredBy core.Color, offeredByUserID string, now time.Time) (DrawOfferState, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.Status != "active" {
		return DrawOfferState{}, errors.New("game is over")
	}

	if s.DrawOffer != nil {
		if now.Before(s.DrawOffer.ExpiresAt) {
			return *s.DrawOffer, ErrDrawOfferAlreadyActive
		}
		s.DrawOffer = nil
	}

	offer := DrawOfferState{
		ID:              uuid.NewString(),
		OfferedBy:       offeredBy,
		OfferedByUserID: offeredByUserID,
		ExpiresAt:       now.Add(DrawOfferTTL),
	}
	s.DrawOffer = &offer
	return offer, nil
}

func (s *GameSession) AcceptDrawOffer(acceptedBy core.Color, now time.Time) (DrawOfferState, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	offer, err := s.activeDrawOfferLocked(acceptedBy, now)
	if err != nil {
		return offer, err
	}

	s.DrawOffer = nil
	return offer, nil
}

func (s *GameSession) DeclineDrawOffer(declinedBy core.Color, now time.Time) (DrawOfferState, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	offer, err := s.activeDrawOfferLocked(declinedBy, now)
	if err != nil {
		return offer, err
	}

	s.DrawOffer = nil
	return offer, nil
}

func (s *GameSession) ExpireDrawOffer(offerID string, now time.Time) (DrawOfferState, bool) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.DrawOffer == nil || s.DrawOffer.ID != offerID || s.Status != "active" {
		return DrawOfferState{}, false
	}
	if now.Before(s.DrawOffer.ExpiresAt) {
		return DrawOfferState{}, false
	}

	offer := *s.DrawOffer
	s.DrawOffer = nil
	return offer, true
}

func (s *GameSession) activeDrawOfferLocked(respondedBy core.Color, now time.Time) (DrawOfferState, error) {
	if s.Status != "active" {
		return DrawOfferState{}, errors.New("game is over")
	}
	if s.DrawOffer == nil {
		return DrawOfferState{}, ErrDrawOfferNotFound
	}

	offer := *s.DrawOffer
	if !now.Before(offer.ExpiresAt) {
		s.DrawOffer = nil
		return offer, ErrDrawOfferExpired
	}
	if offer.OfferedBy == respondedBy {
		return offer, ErrDrawOfferOwnResponse
	}

	return offer, nil
}
