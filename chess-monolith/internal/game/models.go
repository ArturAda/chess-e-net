package game

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Game struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	WhiteID uuid.UUID `gorm:"type:uuid;not null"`
	BlackID uuid.UUID `gorm:"type:uuid;not null"`

	Mode     string `gorm:"type:varchar(50);default:'classic'"`
	IsRanked bool   `gorm:"default:false"`
	Status   string `gorm:"type:varchar(20);default:'active'"` // 'active', 'draw', 'white_won', 'black_won'
	Turn     string `gorm:"type:varchar(10);default:'white'"`

	BoardState string `gorm:"type:text"` // История ходов и текущая расстановка фигур

	WinnerID  *uuid.UUID `gorm:"type:uuid"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (g *Game) BeforeCreate(tx *gorm.DB) (err error) {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return
}
