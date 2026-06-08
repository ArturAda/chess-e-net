package users

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const emailVerificationTTL = time.Minute

func generateVerificationCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", value.Int64()), nil
}

func hashVerificationCode(code string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func verificationCodeMatches(hash string, code string) bool {
	if strings.TrimSpace(hash) == "" || strings.TrimSpace(code) == "" {
		return false
	}

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(strings.TrimSpace(code))) == nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
