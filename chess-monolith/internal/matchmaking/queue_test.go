package matchmaking

import (
	"chess-monolith/internal/game"
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/users"
	"chess-monolith/internal/ws"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// DummyRepo заглушки для тестов
type DummyGameRepo struct{}

func (d *DummyGameRepo) CreateGame(g *game.Game) error                             { return nil }
func (d *DummyGameRepo) GetGame(id uuid.UUID) (*game.Game, error)                  { return nil, nil }
func (d *DummyGameRepo) UpdateGame(id uuid.UUID, state, status, turn string) error { return nil }

type DummyUserRepo struct{}

func (d *DummyUserRepo) CreateUser(u *users.User) error                     { return nil }
func (d *DummyUserRepo) GetUserByEmail(email string) (*users.User, error)   { return nil, nil }
func (d *DummyUserRepo) GetUserByID(id uuid.UUID) (*users.User, error)      { return nil, nil }
func (d *DummyUserRepo) UpdateRatings(wID, lID uuid.UUID, wR, lR int) error { return nil }

type DummyMode struct{}

func (m *DummyMode) Setup() *core.Board                                                   { return core.NewBoard(8, 8) }
func (m *DummyMode) ValidateMove(b *core.Board, turn core.Color, from, to core.Pos) error { return nil }
func (m *DummyMode) ApplyMoveSideEffects(b *core.Board, from, to core.Pos)                {}
func (m *DummyMode) CheckState(b *core.Board, turn core.Color) string                     { return "active" }

func TestMatchmaker_AddRemovePlayer(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("classic", &DummyMode{}) // Заглушка
	mm := NewMatchmaker(reg, &DummyGameRepo{}, &DummyUserRepo{})

	client := &ws.Client{UserID: "user-1"}

	// Добавляем в casual очередь
	mm.AddPlayer(client, "classic", false, 10*time.Minute)

	key := QueueKey{Mode: "classic", TimeLimit: 10 * time.Minute}
	assert.Equal(t, 1, len(mm.casualQueues[key]))

	mm.RemovePlayer(client)
	assert.Equal(t, 0, len(mm.casualQueues[key]))
}

func TestMatchmaker_CasualMatch(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("classic", &DummyMode{})
	mm := NewMatchmaker(reg, &DummyGameRepo{}, &DummyUserRepo{})

	c1 := &ws.Client{UserID: uuid.New().String(), Send: make(chan []byte, 10)}
	c2 := &ws.Client{UserID: uuid.New().String(), Send: make(chan []byte, 10)}

	mm.AddPlayer(c1, "classic", false, 10*time.Minute)
	mm.AddPlayer(c2, "classic", false, 10*time.Minute)

	go mm.Run()
	select {
	case <-c1.Send:
		// Даем воркеру 100мс, чтобы он успел выполнить строку:
		// m.casualQueues[key] = queue[2:]
		time.Sleep(100 * time.Millisecond)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for casual match")
	}

	mm.mu.Lock()
	key := QueueKey{Mode: "classic", TimeLimit: 10 * time.Minute}
	assert.Equal(t, 0, len(mm.casualQueues[key]))
	mm.mu.Unlock()
}

func TestMatchmaker_RankedMatch_Success(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("classic", &DummyMode{})
	mm := NewMatchmaker(reg, &DummyGameRepo{}, &DummyUserRepo{})

	c1 := &ws.Client{UserID: uuid.New().String(), Rating: 1500, Send: make(chan []byte, 10)}
	c2 := &ws.Client{UserID: uuid.New().String(), Rating: 1550, Send: make(chan []byte, 10)}

	mm.AddPlayer(c1, "classic", true, 10*time.Minute)
	mm.AddPlayer(c2, "classic", true, 10*time.Minute)

	go mm.Run()
	select {
	case <-c1.Send:
		time.Sleep(100 * time.Millisecond)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for casual match")
	}

	mm.mu.Lock()
	key := QueueKey{Mode: "classic", TimeLimit: 10 * time.Minute}
	assert.Equal(t, 0, len(mm.rankedQueues[key]))
	mm.mu.Unlock()
}

func TestMatchmaker_RankedMatch_Fail(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("classic", &DummyMode{})
	mm := NewMatchmaker(reg, &DummyGameRepo{}, &DummyUserRepo{})

	c1 := &ws.Client{UserID: uuid.New().String(), Rating: 1500, Send: make(chan []byte, 10)}
	c2 := &ws.Client{UserID: uuid.New().String(), Rating: 1800, Send: make(chan []byte, 10)}

	mm.AddPlayer(c1, "classic", true, 10*time.Minute)
	mm.AddPlayer(c2, "classic", true, 10*time.Minute)

	go mm.Run()
	time.Sleep(1 * time.Second)

	mm.mu.Lock()
	key := QueueKey{Mode: "classic", TimeLimit: 10 * time.Minute}
	assert.Equal(t, 2, len(mm.rankedQueues[key]))
	mm.mu.Unlock()
}
