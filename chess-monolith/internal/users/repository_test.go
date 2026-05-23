package users

import (
	"testing"

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
}
