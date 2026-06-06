package ws

import (
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

type chatStickerDefinition struct {
	ID    string
	Label string
	Src   string
}

var chatStickerCatalog = map[string]chatStickerDefinition{
	"shark":  {ID: "shark", Label: "Shark grin", Src: "images/smiles/shark_grin.png"},
	"bite":   {ID: "bite", Label: "Lip bite", Src: "images/smiles/lip_bite.png"},
	"clown":  {ID: "clown", Label: "Clown", Src: "images/smiles/clown.png"},
	"think":  {ID: "think", Label: "Thinking", Src: "images/smiles/thinking.png"},
	"cry":    {ID: "cry", Label: "Crying", Src: "images/smiles/crying.png"},
	"thumb":  {ID: "thumb", Label: "Thumbs up", Src: "images/smiles/thumbs_up.png"},
	"cheese": {ID: "cheese", Label: "Cheese grin", Src: "images/smiles/cheese_grin.png"},
	"crown":  {ID: "crown", Label: "Crowned", Src: "images/smiles/crowned.png"},
	"dizzy":  {ID: "dizzy", Label: "Dizzy", Src: "images/smiles/dizzy.png"},
	"fire":   {ID: "fire", Label: "On fire", Src: "images/smiles/on_fire.png"},
	"sus":    {ID: "sus", Label: "Suspicious", Src: "images/smiles/suspicious.png"},
	"sleep":  {ID: "sleep", Label: "Sleepy", Src: "images/smiles/sleepy.png"},
	"party":  {ID: "party", Label: "Party", Src: "images/smiles/party.png"},
	"cool":   {ID: "cool", Label: "Cool", Src: "images/smiles/cool.png"},
	"rocket": {ID: "rocket", Label: "Rocket mood", Src: "images/smiles/rocket_mood.png"},
}

func (c *Client) handleChatSticker(req ChatStickerRequest) {
	if !c.isInActiveGameWithOpponent() {
		c.SendError(ErrorCodeNotInGame, "You are not in an active game", true)
		return
	}

	sticker, ok := lookupChatSticker(req.StickerID)
	if !ok {
		c.SendError(ErrorCodeInvalidSticker, "Unknown sticker", true)
		return
	}

	payload := ChatStickerPayload{
		MessageID:      uuid.NewString(),
		GameID:         c.ActiveGame.ID,
		SenderUserID:   c.UserID,
		SenderUsername: c.Username,
		SenderColor:    string(c.Color),
		StickerID:      sticker.ID,
		Label:          sticker.Label,
		Src:            sticker.Src,
		SentAt:         time.Now().UTC(),
	}

	c.sendToPlayers(MessageTypeChatSticker, payload)
	log.Printf("Player %s sent chat sticker %s.", c.UserID, sticker.ID)
}

func lookupChatSticker(stickerID string) (chatStickerDefinition, bool) {
	normalizedID := strings.ToLower(strings.TrimSpace(stickerID))
	if normalizedID == "" {
		return chatStickerDefinition{}, false
	}

	sticker, ok := chatStickerCatalog[normalizedID]
	return sticker, ok
}
