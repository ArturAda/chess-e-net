package users

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	err := db.AutoMigrate(&User{}, &UserRating{})
	if err != nil {
		return nil
	}
	return db
}

func TestRepository_UserLifecycle(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)

	testUser := &User{
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashed_secret",
	}

	t.Run("Create User", func(t *testing.T) {
		err := repo.CreateUser(testUser)
		require.NoError(t, err)
	})

	t.Run("Get Existing User", func(t *testing.T) {
		found, err := repo.GetUserByEmail("test@example.com")
		require.NoError(t, err)

		assert.Equal(t, testUser.Username, found.Username)
		assert.Equal(t, 1200, found.Rating)
	})

	t.Run("Get Non-existent User", func(t *testing.T) {
		_, err := repo.GetUserByEmail("non-existent@mail.com")
		assert.Error(t, err, "Должна возвращаться ошибка, если email не найден")
	})

	t.Run("Get Existing User by ID", func(t *testing.T) {
		foundByEmail, err := repo.GetUserByEmail("test@example.com")
		require.NoError(t, err)

		foundByID, err := repo.GetUserByID(foundByEmail.ID)
		require.NoError(t, err)

		assert.Equal(t, foundByEmail.ID, foundByID.ID)
		assert.Equal(t, foundByEmail.Username, foundByID.Username)
		assert.Equal(t, foundByEmail.Email, foundByID.Email)
	})

	t.Run("Get Non-existent User by ID", func(t *testing.T) {
		randomID := uuid.New()

		_, err := repo.GetUserByID(randomID)

		assert.ErrorIs(t, err, ErrUserNotFound)
	})

	t.Run("Create Duplicate User", func(t *testing.T) {
		// Пытаемся создать юзера с тем же email, что и в первом тесте
		duplicateUser := &User{
			Username:     "another_name",
			Email:        "test@example.com",
			PasswordHash: "some_hash",
		}

		err := repo.CreateUser(duplicateUser)

		// Репозиторий должен перехватить gorm.ErrDuplicatedKey и вернуть ErrUserExists
		assert.ErrorIs(t, err, ErrUserExists)
	})
}

func TestRepository_GetOrCreateRating(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)
	userID := uuid.New()
	require.NoError(t, db.Create(&User{
		ID:           userID,
		Username:     "scoped",
		Email:        "scoped@test.local",
		PasswordHash: "hash",
	}).Error)

	scope := RatingScope{
		Mode:        "classic",
		BoardSize:   8,
		TimeLimitMs: 600000,
	}

	first, err := repo.GetOrCreateRating(userID, scope)
	require.NoError(t, err)
	assert.Equal(t, DefaultRating, first.Rating)
	assert.Equal(t, 0, first.GamesPlayed)
	assert.Equal(t, "classic", first.Mode)
	assert.Equal(t, 8, first.BoardSize)
	assert.Equal(t, int64(600000), first.TimeLimitMs)

	second, err := repo.GetOrCreateRating(userID, scope)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)

	var count int64
	require.NoError(t, db.Model(&UserRating{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestRepository_ApplyRatingResultUsesScope(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)
	user1ID := uuid.New()
	user2ID := uuid.New()
	require.NoError(t, db.Create(&User{
		ID:           user1ID,
		Username:     "winner",
		Email:        "winner@test.local",
		PasswordHash: "hash",
	}).Error)
	require.NoError(t, db.Create(&User{
		ID:           user2ID,
		Username:     "loser",
		Email:        "loser@test.local",
		PasswordHash: "hash",
	}).Error)

	blitzScope := RatingScope{
		Mode:        "classic",
		BoardSize:   8,
		TimeLimitMs: 300000,
	}
	rapidScope := RatingScope{
		Mode:        "classic",
		BoardSize:   8,
		TimeLimitMs: 600000,
	}

	newRating1, newRating2, err := repo.ApplyRatingResult(user1ID, user2ID, blitzScope, 1)
	require.NoError(t, err)
	assert.Equal(t, 1216, newRating1)
	assert.Equal(t, 1184, newRating2)

	user1Blitz, err := repo.GetOrCreateRating(user1ID, blitzScope)
	require.NoError(t, err)
	assert.Equal(t, 1216, user1Blitz.Rating)
	assert.Equal(t, 1, user1Blitz.GamesPlayed)

	user2Blitz, err := repo.GetOrCreateRating(user2ID, blitzScope)
	require.NoError(t, err)
	assert.Equal(t, 1184, user2Blitz.Rating)
	assert.Equal(t, 1, user2Blitz.GamesPlayed)

	user1Rapid, err := repo.GetOrCreateRating(user1ID, rapidScope)
	require.NoError(t, err)
	assert.Equal(t, DefaultRating, user1Rapid.Rating)
	assert.Equal(t, 0, user1Rapid.GamesPlayed)
}

func TestRepository_ListRatingsForUser(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)
	userID := uuid.New()
	otherUserID := uuid.New()
	require.NoError(t, db.Create(&User{
		ID:           userID,
		Username:     "rated",
		Email:        "rated@test.local",
		PasswordHash: "hash",
	}).Error)
	require.NoError(t, db.Create(&User{
		ID:           otherUserID,
		Username:     "other-rated",
		Email:        "other-rated@test.local",
		PasswordHash: "hash",
	}).Error)

	require.NoError(t, db.Create(&UserRating{
		UserID:      userID,
		Mode:        "classic",
		BoardSize:   10,
		TimeLimitMs: 300000,
		Rating:      1250,
		GamesPlayed: 2,
	}).Error)
	require.NoError(t, db.Create(&UserRating{
		UserID:      userID,
		Mode:        "classic",
		BoardSize:   8,
		TimeLimitMs: 600000,
		Rating:      1300,
		GamesPlayed: 3,
	}).Error)
	require.NoError(t, db.Create(&UserRating{
		UserID:      otherUserID,
		Mode:        "classic",
		BoardSize:   8,
		TimeLimitMs: 600000,
		Rating:      1400,
		GamesPlayed: 4,
	}).Error)

	ratings, err := repo.ListRatingsForUser(userID)

	require.NoError(t, err)
	require.Len(t, ratings, 2)
	assert.Equal(t, 8, ratings[0].BoardSize)
	assert.Equal(t, int64(600000), ratings[0].TimeLimitMs)
	assert.Equal(t, 10, ratings[1].BoardSize)
	assert.Equal(t, int64(300000), ratings[1].TimeLimitMs)
}

func TestRepository_ListLeaderboard(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)
	leaderID := uuid.New()
	runnerID := uuid.New()
	otherScopeUserID := uuid.New()
	require.NoError(t, db.Create(&User{
		ID:           leaderID,
		Username:     "leader",
		Email:        "leader@test.local",
		PasswordHash: "hash",
	}).Error)
	require.NoError(t, db.Create(&User{
		ID:           runnerID,
		Username:     "runner",
		Email:        "runner@test.local",
		PasswordHash: "hash",
	}).Error)
	require.NoError(t, db.Create(&User{
		ID:           otherScopeUserID,
		Username:     "other",
		Email:        "other@test.local",
		PasswordHash: "hash",
	}).Error)

	scope := RatingScope{
		Mode:        "classic",
		BoardSize:   8,
		TimeLimitMs: 600000,
	}
	require.NoError(t, db.Create(&UserRating{
		UserID:      runnerID,
		Mode:        scope.Mode,
		BoardSize:   scope.BoardSize,
		TimeLimitMs: scope.TimeLimitMs,
		Rating:      1300,
		GamesPlayed: 3,
	}).Error)
	require.NoError(t, db.Create(&UserRating{
		UserID:      leaderID,
		Mode:        scope.Mode,
		BoardSize:   scope.BoardSize,
		TimeLimitMs: scope.TimeLimitMs,
		Rating:      1400,
		GamesPlayed: 5,
	}).Error)
	require.NoError(t, db.Create(&UserRating{
		UserID:      otherScopeUserID,
		Mode:        scope.Mode,
		BoardSize:   10,
		TimeLimitMs: scope.TimeLimitMs,
		Rating:      1500,
		GamesPlayed: 8,
	}).Error)

	leaderboard, err := repo.ListLeaderboard(scope, 50)

	require.NoError(t, err)
	require.Len(t, leaderboard, 2)
	assert.Equal(t, leaderID, leaderboard[0].UserID)
	assert.Equal(t, "leader", leaderboard[0].Username)
	assert.Equal(t, 1400, leaderboard[0].Rating)
	assert.Equal(t, runnerID, leaderboard[1].UserID)
	assert.Equal(t, "runner", leaderboard[1].Username)
	assert.Equal(t, 1300, leaderboard[1].Rating)
}
