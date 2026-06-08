package users

const (
	defaultRatingMode        = "classic"
	defaultRatingBoardSize   = 8
	defaultRatingTimeLimitMs = int64(10 * 60 * 1000)
)

var supportedRatingBoardSizes = []int{8, 10, 12}
var supportedRatingTimeLimitMinutes = []int{1, 5, 10, 30}

type ratingScopeMapKey struct {
	Mode        string
	BoardSize   int
	TimeLimitMs int64
}

func supportedRatingScopes() []RatingScope {
	scopes := make([]RatingScope, 0, len(supportedRatingBoardSizes)*len(supportedRatingTimeLimitMinutes))
	for _, boardSize := range supportedRatingBoardSizes {
		for _, minutes := range supportedRatingTimeLimitMinutes {
			scopes = append(scopes, RatingScope{
				Mode:        defaultRatingMode,
				BoardSize:   boardSize,
				TimeLimitMs: minutesToMilliseconds(minutes),
			})
		}
	}

	return scopes
}

func BoardRatingScope(boardSize int, timeLimitMs int64) RatingScope {
	return normalizeRatingScope(RatingScope{
		Mode:        defaultRatingMode,
		BoardSize:   boardSize,
		TimeLimitMs: timeLimitMs,
	})
}

func isSupportedRatingScope(scope RatingScope) bool {
	scope = normalizeRatingScope(scope)
	if scope.Mode != defaultRatingMode {
		return false
	}
	for _, supportedScope := range supportedRatingScopes() {
		if ratingScopeKey(supportedScope) == ratingScopeKey(scope) {
			return true
		}
	}

	return false
}

func ratingScopeDTO(scope RatingScope) RatingScopeDTO {
	scope = normalizeRatingScope(scope)
	return RatingScopeDTO{
		Mode:             scope.Mode,
		BoardSize:        scope.BoardSize,
		TimeLimitMs:      scope.TimeLimitMs,
		TimeLimitMinutes: int(scope.TimeLimitMs / (60 * 1000)),
	}
}

func userRatingDTO(rating UserRating) UserRatingDTO {
	return UserRatingDTO{
		RatingScopeDTO: ratingScopeDTO(RatingScope{
			Mode:        rating.Mode,
			BoardSize:   rating.BoardSize,
			TimeLimitMs: rating.TimeLimitMs,
		}),
		Rating:      rating.Rating,
		GamesPlayed: rating.GamesPlayed,
	}
}

func defaultUserRatingDTO(scope RatingScope) UserRatingDTO {
	return UserRatingDTO{
		RatingScopeDTO: ratingScopeDTO(scope),
		Rating:         DefaultRating,
		GamesPlayed:    0,
	}
}

func ratingScopeKey(scope RatingScope) ratingScopeMapKey {
	scope = normalizeRatingScope(scope)
	return ratingScopeMapKey{
		Mode:        scope.Mode,
		BoardSize:   scope.BoardSize,
		TimeLimitMs: scope.TimeLimitMs,
	}
}

func minutesToMilliseconds(minutes int) int64 {
	return int64(minutes) * 60 * 1000
}
