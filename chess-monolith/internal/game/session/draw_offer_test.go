package session

import (
	"chess-monolith/internal/game/core"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDrawOfferLifecycle(t *testing.T) {
	game := &GameSession{Status: "active"}
	now := time.Now()

	offer, err := game.CreateDrawOffer(core.White, "white-user", now)
	require.NoError(t, err)
	assert.Equal(t, core.White, offer.OfferedBy)
	assert.Equal(t, "white-user", offer.OfferedByUserID)
	assert.Equal(t, now.Add(DrawOfferTTL), offer.ExpiresAt)
	require.NotNil(t, game.DrawOffer)

	acceptedOffer, err := game.AcceptDrawOffer(core.Black, now.Add(10*time.Second))
	require.NoError(t, err)
	assert.Equal(t, offer.ID, acceptedOffer.ID)
	assert.Nil(t, game.DrawOffer)
}

func TestCreateDrawOfferRejectsActiveOffer(t *testing.T) {
	game := &GameSession{Status: "active"}
	now := time.Now()

	offer, err := game.CreateDrawOffer(core.White, "white-user", now)
	require.NoError(t, err)

	activeOffer, err := game.CreateDrawOffer(core.Black, "black-user", now.Add(10*time.Second))
	assert.True(t, errors.Is(err, ErrDrawOfferAlreadyActive))
	assert.Equal(t, offer.ID, activeOffer.ID)
}

func TestDrawOfferCannotBeAcceptedByOfferingPlayer(t *testing.T) {
	game := &GameSession{Status: "active"}
	now := time.Now()

	offer, err := game.CreateDrawOffer(core.White, "white-user", now)
	require.NoError(t, err)

	activeOffer, err := game.AcceptDrawOffer(core.White, now.Add(10*time.Second))
	assert.True(t, errors.Is(err, ErrDrawOfferOwnResponse))
	assert.Equal(t, offer.ID, activeOffer.ID)
	require.NotNil(t, game.DrawOffer)
	assert.Equal(t, offer.ID, game.DrawOffer.ID)
}

func TestDrawOfferExpires(t *testing.T) {
	game := &GameSession{Status: "active"}
	now := time.Now()

	offer, err := game.CreateDrawOffer(core.White, "white-user", now)
	require.NoError(t, err)

	expiredOffer, err := game.AcceptDrawOffer(core.Black, now.Add(DrawOfferTTL+time.Millisecond))
	assert.True(t, errors.Is(err, ErrDrawOfferExpired))
	assert.Equal(t, offer.ID, expiredOffer.ID)
	assert.Nil(t, game.DrawOffer)
}

func TestDeclineDrawOfferClearsOffer(t *testing.T) {
	game := &GameSession{Status: "active"}
	now := time.Now()

	offer, err := game.CreateDrawOffer(core.White, "white-user", now)
	require.NoError(t, err)

	declinedOffer, err := game.DeclineDrawOffer(core.Black, now.Add(10*time.Second))
	require.NoError(t, err)
	assert.Equal(t, offer.ID, declinedOffer.ID)
	assert.Nil(t, game.DrawOffer)
}

func TestExpireDrawOfferIgnoresWrongID(t *testing.T) {
	game := &GameSession{Status: "active"}
	now := time.Now()

	offer, err := game.CreateDrawOffer(core.White, "white-user", now)
	require.NoError(t, err)

	_, expired := game.ExpireDrawOffer("other-offer", now.Add(DrawOfferTTL+time.Millisecond))
	assert.False(t, expired)
	require.NotNil(t, game.DrawOffer)
	assert.Equal(t, offer.ID, game.DrawOffer.ID)
}
