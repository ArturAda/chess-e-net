// Файл: pkg/elo/elo.go
package elo

import "math"

const K = 32.0

// Calculate возвращает новые рейтинги для Игрока 1 и Игрока 2
// score1: 1.0 (победа первого), 0.0 (поражение), 0.5 (ничья)
func Calculate(rating1, rating2 int, score1 float64) (int, int) {
	r1 := float64(rating1)
	r2 := float64(rating2)

	// Ожидаемый результат для первого игрока
	expected1 := 1.0 / (1.0 + math.Pow(10.0, (r2-r1)/400.0))
	// Ожидаемый результат для второго игрока
	expected2 := 1.0 - expected1

	score2 := 1.0 - score1

	newRating1 := int(math.Round(r1 + K*(score1-expected1)))
	newRating2 := int(math.Round(r2 + K*(score2-expected2)))

	// Защита от падения рейтинга ниже 100
	if newRating1 < 100 {
		newRating1 = 100
	}
	if newRating2 < 100 {
		newRating2 = 100
	}

	return newRating1, newRating2
}
