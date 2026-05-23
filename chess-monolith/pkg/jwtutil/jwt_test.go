package jwtutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWT_FullCycle(t *testing.T) {
	secret := "super_secret_key"
	userID := "550e8400-e29b-41d4-a716-446655440000"

	t.Run("Happy Path - Generate and Parse", func(t *testing.T) {
		token, err := GenerateToken(userID, secret)
		require.NoError(t, err) // require прерывает подтест при ошибке генерации
		require.NotEmpty(t, token)

		parsedID, err := ParseToken(token, secret)
		assert.NoError(t, err)
		assert.Equal(t, userID, parsedID)
	})

	t.Run("Invalid Secret", func(t *testing.T) {
		wrongSecret := "wrong_secret"

		token, err := GenerateToken(userID, secret)
		require.NoError(t, err)

		_, err = ParseToken(token, wrongSecret)
		assert.Error(t, err, "Парсинг должен падать при неверном секрете")
	})

	t.Run("Malformed Token", func(t *testing.T) {
		notAToken := "not.a.token"

		_, err := ParseToken(notAToken, secret)
		assert.Error(t, err, "Парсинг должен падать на невалидной строке")
	})
}
