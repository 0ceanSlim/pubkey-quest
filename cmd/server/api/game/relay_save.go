package game

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/savecodec"
	"pubkey-quest/cmd/server/ledger"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/utils"
)

// ─── Relay saves: preparing the event (docs/draft/relay-saves-plan.md §2) ────
//
// With relay saves on, "Save" doesn't write anything yet. The server builds the
// exact event content from the live session and hands it to the browser to
// sign; /api/saves/publish then checks the signed event matches, broadcasts it,
// and only on a relay's OK writes the disk cache and the ledger.

// SaveKind is the event kind saves use (NIP-78, app-specific data).
const SaveKind = 30078

// MaxSaveBytes caps the save content (relays commonly cap events at 16–64 KB).
const MaxSaveBytes = 16 << 10

// ClientTag marks events this game wrote (NIP-89), matching the profile editor.
var ClientTag = []string{"client", "Pubkey Quest"}

// SaveDTag is the d tag for a character's save slot.
func SaveDTag(saveID string) string { return "pubkey-quest:save:" + saveID }

// SaveIDFromDTag is the inverse of SaveDTag ("" when it isn't one of ours).
func SaveIDFromDTag(d string) string {
	if !strings.HasPrefix(d, "pubkey-quest:save:") {
		return ""
	}
	return strings.TrimPrefix(d, "pubkey-quest:save:")
}

// UnsignedSave is the event the browser signs.
// swagger:model UnsignedSave
type UnsignedSave struct {
	Kind      int        `json:"kind"`
	CreatedAt int64      `json:"created_at"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
}

// prepareRelaySave builds the save event for the session and remembers it as
// pending.
func prepareRelaySave(sess *session.GameSession, saveID string) (*UnsignedSave, error) {
	if why := saveBlocked(sess); why != "" {
		return nil, fmt.Errorf("%s", why)
	}
	content, err := savecodec.Encode(&sess.SaveData)
	if err != nil {
		return nil, err
	}
	if len(content) > MaxSaveBytes {
		return nil, fmt.Errorf("this save is %d bytes, over the %d-byte limit", len(content), MaxSaveBytes)
	}
	d := SaveDTag(saveID)
	s := &sess.SaveData
	level := character.GetLevelFromXP(s.Experience, cachedAdvancement())
	tags := [][]string{
		{"d", d},
		ClientTag,
		{"v", strconv.Itoa(savecodec.Version)},
		{"build", utils.AppVersion}, // the game version that wrote it (support / debugging)
		{"alt", fmt.Sprintf("Pubkey Quest save — %s, level %d %s %s", s.D, level, s.Race, s.Class)},
	}
	if e, ok := ledger.Get(sess.Npub, d); ok {
		tags = append(tags, []string{"prev", e.EventID})
	}
	disk, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	sess.PendingSave = &session.PendingSave{D: d, Content: string(content), Tags: tags, CreatedAt: now, DiskJSON: disk}
	return &UnsignedSave{Kind: SaveKind, CreatedAt: now, Tags: tags, Content: string(content)}, nil
}
