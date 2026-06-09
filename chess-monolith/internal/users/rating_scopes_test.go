package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRatingScopeHelpers(t *testing.T) {
	scopes := supportedRatingScopes()
	assert.Len(t, scopes, len(supportedRatingBoardSizes)*len(supportedRatingTimeLimitMinutes))
	assert.Contains(t, scopes, RatingScope{Mode: "classic", BoardSize: 8, TimeLimitMs: minutesToMilliseconds(1)})
	assert.Contains(t, scopes, RatingScope{Mode: "classic", BoardSize: 12, TimeLimitMs: minutesToMilliseconds(30)})

	assert.True(t, isSupportedRatingScope(RatingScope{BoardSize: 8, TimeLimitMs: minutesToMilliseconds(10)}))
	assert.False(t, isSupportedRatingScope(RatingScope{Mode: "custom", BoardSize: 8, TimeLimitMs: minutesToMilliseconds(10)}))
	assert.False(t, isSupportedRatingScope(RatingScope{BoardSize: 14, TimeLimitMs: minutesToMilliseconds(10)}))
	assert.False(t, isSupportedRatingScope(RatingScope{BoardSize: 8, TimeLimitMs: minutesToMilliseconds(2)}))

	defaulted := BoardRatingScope(0, 0)
	assert.Equal(t, RatingScope{
		Mode:        "classic",
		BoardSize:   8,
		TimeLimitMs: minutesToMilliseconds(10),
	}, defaulted)

	scope := BoardRatingScope(10, minutesToMilliseconds(5))
	assert.Equal(t, ratingScopeMapKey{Mode: "classic", BoardSize: 10, TimeLimitMs: minutesToMilliseconds(5)}, ratingScopeKey(scope))

	scopeDTO := ratingScopeDTO(scope)
	assert.Equal(t, "classic", scopeDTO.Mode)
	assert.Equal(t, 10, scopeDTO.BoardSize)
	assert.Equal(t, int64(300000), scopeDTO.TimeLimitMs)
	assert.Equal(t, 5, scopeDTO.TimeLimitMinutes)

	ratingDTO := userRatingDTO(UserRating{
		Mode:        "classic",
		BoardSize:   12,
		TimeLimitMs: minutesToMilliseconds(30),
		Rating:      1250,
		GamesPlayed: 7,
	})
	assert.Equal(t, 1250, ratingDTO.Rating)
	assert.Equal(t, 7, ratingDTO.GamesPlayed)
	assert.Equal(t, 30, ratingDTO.TimeLimitMinutes)

	fallbackDTO := defaultUserRatingDTO(RatingScope{})
	assert.Equal(t, DefaultRating, fallbackDTO.Rating)
	assert.Equal(t, 0, fallbackDTO.GamesPlayed)
	assert.Equal(t, 8, fallbackDTO.BoardSize)
	assert.Equal(t, 10, fallbackDTO.TimeLimitMinutes)
}
