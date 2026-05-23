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
	err := db.AutoMigrate(&User{})
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
