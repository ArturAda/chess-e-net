package matchmaking

import (
	"chess-monolith/internal/game"
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/users"
	"chess-monolith/internal/ws"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DummyRepo заглушки для тестов
type DummyGameRepo struct {
	Created []*game.Game
}

func (d *DummyGameRepo) CreateGame(g *game.Game) error {
	d.Created = append(d.Created, g)
	return nil
}
func (d *DummyGameRepo) GetGame(id uuid.UUID) (*game.Game, error)                  { return nil, nil }
func (d *DummyGameRepo) GetGameForUser(id, userID uuid.UUID) (*game.Game, error)   { return nil, nil }
func (d *DummyGameRepo) ListGamesForUser(userID uuid.UUID) ([]game.Game, error)    { return nil, nil }
func (d *DummyGameRepo) UpdateGame(id uuid.UUID, state, status, turn string) error { return nil }

type DummyUserRepo struct {
	Ratings map[uuid.UUID]int
}

func (d *DummyUserRepo) CreateUser(u *users.User) error                     { return nil }
func (d *DummyUserRepo) GetUserByEmail(email string) (*users.User, error)   { return nil, nil }
func (d *DummyUserRepo) GetUserByID(id uuid.UUID) (*users.User, error)      { return nil, nil }
func (d *DummyUserRepo) UpdateRatings(wID, lID uuid.UUID, wR, lR int) error { return nil }
func (d *DummyUserRepo) GetOrCreateRating(userID uuid.UUID, _ users.RatingScope) (*users.UserRating, error) {
	rating := users.DefaultRating
	if d.Ratings != nil {
		if scopedRating, ok := d.Ratings[userID]; ok {
			rating = scopedRating
		}
	}
	return &users.UserRating{Rating: rating}, nil
}
func (d *DummyUserRepo) ListRatingsForUser(_ uuid.UUID) ([]users.UserRating, error) {
	return nil, nil
}
func (d *DummyUserRepo) ListLeaderboard(_ users.RatingScope, _ int) ([]users.LeaderboardEntry, error) {
	return nil, nil
}
func (d *DummyUserRepo) ApplyRatingResult(_ uuid.UUID, _ uuid.UUID, _ users.RatingScope, _ float64) (int, int, error) {
	return 1216, 1184, nil
}

type DummyMode struct {
	Size int
}

func (m *DummyMode) Setup() *core.Board {
	size := m.Size
	if size <= 0 {
		size = 8
	}
	return core.NewBoard(size, size)
}
func (m *DummyMode) ValidateMove(b *core.Board, turn core.Color, from, to core.Pos) error { return nil }
func (m *DummyMode) ApplyMoveSideEffects(b *core.Board, from, to core.Pos)                {}
func (m *DummyMode) CheckState(b *core.Board, turn core.Color) string                     { return "active" }

func readClientMessage(t *testing.T, client *ws.Client) ws.Message {
	t.Helper()

	select {
	case raw := <-client.Send:
		var msg ws.Message
		require.NoError(t, json.Unmarshal(raw, &msg))
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for websocket message")
	}

	return ws.Message{}
}

func TestMatchmaker_AddRemovePlayer(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("classic", &DummyMode{}) // Заглушка
	mm := NewMatchmaker(reg, &DummyGameRepo{}, &DummyUserRepo{})

	client := &ws.Client{UserID: "user-1"}

	// Добавляем в casual очередь
	require.NoError(t, mm.AddPlayer(client, "classic", 8, false, 10*time.Minute))

	key := QueueKey{Mode: "classic", BoardSize: 8, TimeLimit: 10 * time.Minute}
	assert.Equal(t, 1, len(mm.casualQueues[key]))

	mm.RemovePlayer(client)
	assert.Equal(t, 0, len(mm.casualQueues[key]))
}

func TestMatchmaker_AddPlayer_UnknownMode(t *testing.T) {
	reg := core.NewRegistry()
	mm := NewMatchmaker(reg, &DummyGameRepo{}, &DummyUserRepo{})

	client := &ws.Client{UserID: "user-1"}

	err := mm.AddPlayer(client, "unknown", 8, false, 10*time.Minute)
	require.Error(t, err)

	var protocolErr *ws.ProtocolError
	require.ErrorAs(t, err, &protocolErr)
	assert.Equal(t, ws.ErrorCodeUnknownMode, protocolErr.Code)
	assert.True(t, protocolErr.Recoverable)
	assert.Empty(t, mm.casualQueues)
}

func TestMatchmaker_AddPlayer_ModernMode(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("modern12", &DummyMode{Size: 12})
	mm := NewMatchmaker(reg, &DummyGameRepo{}, &DummyUserRepo{})

	client := &ws.Client{UserID: "user-1"}

	require.NoError(t, mm.AddPlayer(client, "modern12", 12, false, 30*time.Minute))

	key := QueueKey{Mode: "modern12", BoardSize: 12, TimeLimit: 30 * time.Minute}
	assert.Len(t, mm.casualQueues[key], 1)
}

func TestMatchmaker_CasualMatch(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("classic", &DummyMode{})
	mm := NewMatchmaker(reg, &DummyGameRepo{}, &DummyUserRepo{})

	c1 := &ws.Client{UserID: uuid.New().String(), Send: make(chan []byte, 10)}
	c2 := &ws.Client{UserID: uuid.New().String(), Send: make(chan []byte, 10)}

	require.NoError(t, mm.AddPlayer(c1, "classic", 8, false, 10*time.Minute))
	require.NoError(t, mm.AddPlayer(c2, "classic", 8, false, 10*time.Minute))

	go mm.Run()

	c1Match := readClientMessage(t, c1)
	assert.Equal(t, ws.MessageTypeMatchFound, c1Match.Type)

	c1State := readClientMessage(t, c1)
	assert.Equal(t, ws.MessageTypeGameState, c1State.Type)

	c2Match := readClientMessage(t, c2)
	assert.Equal(t, ws.MessageTypeMatchFound, c2Match.Type)

	c2State := readClientMessage(t, c2)
	assert.Equal(t, ws.MessageTypeGameState, c2State.Type)

	var c1MatchPayload ws.MatchFoundPayload
	require.NoError(t, json.Unmarshal(c1Match.Payload, &c1MatchPayload))
	assert.Equal(t, "white", c1MatchPayload.PlayerColor)
	assert.Equal(t, "classic", c1MatchPayload.Mode)
	assert.Equal(t, 8, c1MatchPayload.BoardSize)

	var c2MatchPayload ws.MatchFoundPayload
	require.NoError(t, json.Unmarshal(c2Match.Payload, &c2MatchPayload))
	assert.Equal(t, "black", c2MatchPayload.PlayerColor)

	var c1StatePayload struct {
		PlayerColor string `json:"player_color"`
		BoardSize   int    `json:"board_size"`
	}
	require.NoError(t, json.Unmarshal(c1State.Payload, &c1StatePayload))
	assert.Equal(t, "white", c1StatePayload.PlayerColor)
	assert.Equal(t, 8, c1StatePayload.BoardSize)

	var c2StatePayload struct {
		PlayerColor string `json:"player_color"`
		BoardSize   int    `json:"board_size"`
	}
	require.NoError(t, json.Unmarshal(c2State.Payload, &c2StatePayload))
	assert.Equal(t, "black", c2StatePayload.PlayerColor)
	assert.Equal(t, 8, c2StatePayload.BoardSize)

	// Даем воркеру 100мс, чтобы он успел выполнить строку:
	// m.casualQueues[key] = queue[2:]
	time.Sleep(100 * time.Millisecond)

	mm.mu.Lock()
	key := QueueKey{Mode: "classic", BoardSize: 8, TimeLimit: 10 * time.Minute}
	assert.Equal(t, 0, len(mm.casualQueues[key]))
	mm.mu.Unlock()
}

func TestMatchmaker_StartGamePersistsPlayerVisualStates(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("classic", &DummyMode{})
	repo := &DummyGameRepo{}
	mm := NewMatchmaker(reg, repo, &DummyUserRepo{})

	c1 := &ws.Client{
		UserID:      uuid.New().String(),
		Send:        make(chan []byte, 10),
		VisualState: `{"light_square":{"id":"classic-green"},"pieces":{"white":"pixel"}}`,
	}
	c2 := &ws.Client{
		UserID:      uuid.New().String(),
		Send:        make(chan []byte, 10),
		VisualState: `{"light_square":{"id":"red"},"pieces":{"black":"neo"}}`,
	}

	mm.startGame(c1, c2, "classic", 8, false, 10*time.Minute)

	require.Len(t, repo.Created, 1)
	assert.JSONEq(t, c1.VisualState, repo.Created[0].WhiteVisualState)
	assert.JSONEq(t, c2.VisualState, repo.Created[0].BlackVisualState)
}

func TestMatchmaker_StartGameSupportsModernBoardSize(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("modern10", &DummyMode{Size: 10})
	repo := &DummyGameRepo{}
	mm := NewMatchmaker(reg, repo, &DummyUserRepo{})

	c1 := &ws.Client{UserID: uuid.New().String(), Send: make(chan []byte, 10)}
	c2 := &ws.Client{UserID: uuid.New().String(), Send: make(chan []byte, 10)}

	mm.startGame(c1, c2, "modern10", 10, false, 5*time.Minute)

	require.Len(t, repo.Created, 1)
	assert.Equal(t, "modern10", repo.Created[0].Mode)
	assert.Equal(t, 10, repo.Created[0].BoardSize)
	assert.Equal(t, (5 * time.Minute).Milliseconds(), repo.Created[0].TimeLimitMs)

	c1Match := readClientMessage(t, c1)
	assert.Equal(t, ws.MessageTypeMatchFound, c1Match.Type)
	var matchPayload ws.MatchFoundPayload
	require.NoError(t, json.Unmarshal(c1Match.Payload, &matchPayload))
	assert.Equal(t, "modern10", matchPayload.Mode)
	assert.Equal(t, 10, matchPayload.BoardSize)

	c1State := readClientMessage(t, c1)
	assert.Equal(t, ws.MessageTypeGameState, c1State.Type)
	var statePayload struct {
		BoardSize int `json:"board_size"`
		Board     struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"board"`
	}
	require.NoError(t, json.Unmarshal(c1State.Payload, &statePayload))
	assert.Equal(t, 10, statePayload.BoardSize)
	assert.Equal(t, 10, statePayload.Board.Width)
	assert.Equal(t, 10, statePayload.Board.Height)
}

func TestMatchmaker_RankedMatch_Success(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("classic", &DummyMode{})
	mm := NewMatchmaker(reg, &DummyGameRepo{}, &DummyUserRepo{})

	c1ID := uuid.New()
	c2ID := uuid.New()
	mm.userRepo = &DummyUserRepo{Ratings: map[uuid.UUID]int{
		c1ID: 1500,
		c2ID: 1550,
	}}

	c1 := &ws.Client{UserID: c1ID.String(), Rating: 1500, Send: make(chan []byte, 10)}
	c2 := &ws.Client{UserID: c2ID.String(), Rating: 1550, Send: make(chan []byte, 10)}

	require.NoError(t, mm.AddPlayer(c1, "classic", 8, true, 10*time.Minute))
	require.NoError(t, mm.AddPlayer(c2, "classic", 8, true, 10*time.Minute))

	go mm.Run()

	c1Match := readClientMessage(t, c1)
	assert.Equal(t, ws.MessageTypeMatchFound, c1Match.Type)
	c2Match := readClientMessage(t, c2)
	assert.Equal(t, ws.MessageTypeMatchFound, c2Match.Type)

	var c1MatchPayload ws.MatchFoundPayload
	require.NoError(t, json.Unmarshal(c1Match.Payload, &c1MatchPayload))
	assert.True(t, c1MatchPayload.IsRanked)

	var c2MatchPayload ws.MatchFoundPayload
	require.NoError(t, json.Unmarshal(c2Match.Payload, &c2MatchPayload))
	assert.True(t, c2MatchPayload.IsRanked)

	assert.Equal(t, ws.MessageTypeGameState, readClientMessage(t, c1).Type)
	assert.Equal(t, ws.MessageTypeGameState, readClientMessage(t, c2).Type)

	time.Sleep(100 * time.Millisecond)

	mm.mu.Lock()
	key := QueueKey{Mode: "classic", BoardSize: 8, TimeLimit: 10 * time.Minute}
	assert.Equal(t, 0, len(mm.rankedQueues[key]))
	mm.mu.Unlock()
}

func TestMatchmaker_RankedMatch_Fail(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("classic", &DummyMode{})
	mm := NewMatchmaker(reg, &DummyGameRepo{}, &DummyUserRepo{})

	c1ID := uuid.New()
	c2ID := uuid.New()
	mm.userRepo = &DummyUserRepo{Ratings: map[uuid.UUID]int{
		c1ID: 1500,
		c2ID: 1800,
	}}

	c1 := &ws.Client{UserID: c1ID.String(), Rating: 1500, Send: make(chan []byte, 10)}
	c2 := &ws.Client{UserID: c2ID.String(), Rating: 1800, Send: make(chan []byte, 10)}

	require.NoError(t, mm.AddPlayer(c1, "classic", 8, true, 10*time.Minute))
	require.NoError(t, mm.AddPlayer(c2, "classic", 8, true, 10*time.Minute))

	go mm.Run()
	time.Sleep(1 * time.Second)

	mm.mu.Lock()
	key := QueueKey{Mode: "classic", BoardSize: 8, TimeLimit: 10 * time.Minute}
	assert.Equal(t, 2, len(mm.rankedQueues[key]))
	mm.mu.Unlock()
}
