package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/core/tools"
	grainsession "github.com/0ceanslim/grain/client/session"
	nostr "github.com/0ceanslim/grain/server/types"

	"pubkey-quest/cmd/server/api/game"
	"pubkey-quest/cmd/server/game/savecodec"
	"pubkey-quest/cmd/server/ledger"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/utils"
	"pubkey-quest/types"
)

// ─── Saves on relays (docs/draft/relay-saves-plan.md) ────────────────────────
//
// Saving: the session handler prepares the event; the browser signs it;
// /api/saves/publish checks it is exactly what was prepared, broadcasts it to
// the player's write relays, and only when a relay answers OK writes the disk
// cache (the save JSON + the signed event beside it) and the ledger.
//
// Loading (session.SaveSource): the relay copy is the source of truth.
//   - Official (validation on): only the save the ledger accepted. A matching
//     disk cache is used as is; otherwise the event is fetched by id from the
//     relays that accepted it.
//   - Test / modded (validation off): the newest valid event for the slot,
//     falling back to the disk cache.

// eventPath is the signed event cached beside a save file.
func eventPath(npub, saveID string) string {
	return filepath.Join(session.SavesDirectory, npub, saveID+".event.json")
}

func readCachedEvent(npub, saveID string) *nostr.Event {
	data, err := os.ReadFile(eventPath(npub, saveID))
	if err != nil {
		return nil
	}
	var ev nostr.Event
	if json.Unmarshal(data, &ev) != nil {
		return nil
	}
	return &ev
}

func tagValue(ev *nostr.Event, name string) string {
	for _, t := range ev.Tags {
		if len(t) >= 2 && t[0] == name {
			return t[1]
		}
	}
	return ""
}

// PublishSaveHandler godoc
// @Summary      Publish a signed save
// @Description  The browser signs the event prepared by POST /api/session/save and sends it here. It must
// @Description  match what was prepared exactly. It is broadcast to the player's write relays; on at least one
// @Description  relay OK the save is kept (disk cache + ledger). No OK = no save.
// @Tags         Saves
// @Accept       json
// @Produce      json
// @Router       /api/saves/publish [post]
func PublishSaveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := grainsession.SessionMgr.GetCurrentUser(r)
	if user == nil {
		http.Error(w, "Not signed in", http.StatusUnauthorized)
		return
	}
	var req struct {
		Npub   string       `json:"npub"`
		SaveID string       `json:"save_id"`
		Event  *nostr.Event `json:"event"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&req); err != nil || req.Event == nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Missing signed save"})
		return
	}
	fail := func(status int, msg string) {
		writeJSONStatus(w, status, map[string]any{"success": false, "error": msg})
	}
	sess, err := session.GetSessionManager().GetSession(req.Npub, req.SaveID)
	if err != nil {
		fail(http.StatusNotFound, "Session not found")
		return
	}
	pending := sess.PendingSave
	ev := req.Event
	switch {
	case pending == nil:
		fail(http.StatusConflict, "Nothing is waiting to be saved — press Save again")
		return
	case ev.PubKey != user.PublicKey:
		fail(http.StatusForbidden, "That save was signed by a different key")
		return
	case ev.Kind != game.SaveKind || tagValue(ev, "d") != pending.D:
		fail(http.StatusBadRequest, "That isn't this character's save")
		return
	case ev.Content != pending.Content:
		fail(http.StatusBadRequest, "The signed save doesn't match the game — press Save again")
		return
	case !core.VerifyEventSignature(ev):
		fail(http.StatusBadRequest, "The save's signature doesn't check out")
		return
	}
	// Tags must be exactly what was prepared (client tag, version, prev chain).
	if !sameTags(ev.Tags, pending.Tags) {
		fail(http.StatusBadRequest, "The save's tags were changed — press Save again")
		return
	}

	uc := connection.UserFor(user.PublicKey)
	if uc == nil {
		fail(http.StatusServiceUnavailable, "Relay client not ready — try again in a moment")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	relays := uc.RoutePublish(ev)
	results, _ := uc.PublishEvent(ctx, ev, relays)
	var accepted, refused []string
	for _, res := range results {
		if res.Accepted {
			accepted = append(accepted, res.RelayURL)
		} else {
			reason := res.Reason
			if reason == "" && res.Error != nil {
				reason = res.Error.Error()
			}
			refused = append(refused, fmt.Sprintf("%s (%s)", res.RelayURL, reason))
		}
	}
	if len(accepted) == 0 {
		log.Printf("⚠️ Save for %s:%s not stored by any relay: %v", req.Npub, req.SaveID, refused)
		fail(http.StatusBadGateway, "No relay stored your save, so it didn't happen. Check your relays and try again.")
		return
	}

	// Kept: the disk cache, the signed event beside it, and the ledger.
	dir := filepath.Join(session.SavesDirectory, req.Npub)
	if err := os.MkdirAll(dir, 0o755); err == nil {
		os.WriteFile(session.GetSavePath(req.Npub, req.SaveID), pending.DiskJSON, 0o644)
		if raw, err := json.MarshalIndent(ev, "", "  "); err == nil {
			os.WriteFile(eventPath(req.Npub, req.SaveID), raw, 0o644)
		}
	}
	if err := ledger.Record(req.Npub, pending.D, ledger.Entry{
		EventID: ev.ID, Relays: accepted, CreatedAt: ev.CreatedAt, Version: savecodec.Version,
	}); err != nil {
		log.Printf("❌ Ledger write failed for %s: %v", req.Npub, err)
	}
	sess.PendingSave = nil
	session.RemoveJournal(req.Npub, req.SaveID)
	log.Printf("✅ Signed save %s… for %s:%s stored on %d/%d relays", shortID(ev.ID), req.Npub, req.SaveID, len(accepted), len(results))
	writeJSON(w, map[string]any{"success": true, "event_id": ev.ID, "relays": accepted, "refused": refused})
}

func sameTags(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

// ─── Loading ─────────────────────────────────────────────────────────────────

// fetchEvents queries the player's relays (plus extra) for save events.
func fetchEvents(pubkey string, filter nostr.Filter, extra []string, timeout time.Duration) []*nostr.Event {
	c := connection.GetCoreClient()
	if c == nil {
		return nil
	}
	relays := append([]string{}, extra...)
	relays = append(relays, c.OwnListRelays(pubkey)...)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.QueryEvents(ctx, filter, relays, core.WithTimeout(timeout))
}

// decodeSaveEvent checks a fetched save event and decodes it.
func decodeSaveEvent(ev *nostr.Event, pubkey, d string) (*types.SaveFile, error) {
	if ev == nil || ev.Kind != game.SaveKind || ev.PubKey != pubkey || tagValue(ev, "d") != d {
		return nil, fmt.Errorf("not this character's save")
	}
	if !core.VerifyEventSignature(ev) {
		return nil, fmt.Errorf("bad signature")
	}
	return savecodec.Decode([]byte(ev.Content))
}

// cacheSave writes a relay save to the disk cache.
func cacheSave(npub, saveID string, save *types.SaveFile, ev *nostr.Event) {
	os.MkdirAll(filepath.Join(session.SavesDirectory, npub), 0o755)
	session.WriteSaveFile(session.GetSavePath(npub, saveID), save)
	if raw, err := json.MarshalIndent(ev, "", "  "); err == nil {
		os.WriteFile(eventPath(npub, saveID), raw, 0o644)
	}
}

// relaySaveSource is session.SaveSource with relay saves on.
func relaySaveSource(npub, saveID string) (*types.SaveFile, error) {
	pubkey, err := tools.DecodeNpub(npub)
	if err != nil {
		return nil, err
	}
	d := game.SaveDTag(saveID)
	cached, cacheErr := session.LoadSaveByID(npub, saveID)
	cachedEv := readCachedEvent(npub, saveID)

	if utils.AppConfig.Saves.Validation {
		entry, ok := ledger.Get(npub, d)
		if !ok {
			// A new character that hasn't been saved yet is playable from the
			// server-made disk copy; anything with a signed history must match.
			if cacheErr == nil && cachedEv == nil {
				return cached, nil
			}
			return nil, fmt.Errorf("this server has no record of that save")
		}
		if cacheErr == nil && cachedEv != nil && cachedEv.ID == entry.EventID {
			return cached, nil
		}
		evs := fetchEvents(pubkey, nostr.Filter{IDs: []string{entry.EventID}}, entry.Relays, 8*time.Second)
		for _, ev := range evs {
			if ev.ID != entry.EventID {
				continue
			}
			save, err := decodeSaveEvent(ev, pubkey, d)
			if err != nil {
				return nil, err
			}
			save.InternalID, save.InternalNpub = saveID, npub
			cacheSave(npub, saveID, save, ev)
			return save, nil
		}
		return nil, fmt.Errorf("your last save here (%s…) couldn't be fetched from %v", shortID(entry.EventID), entry.Relays)
	}

	// Test / modded: the newest signed copy wins over the cache.
	evs := fetchEvents(pubkey, nostr.Filter{Authors: []string{pubkey}, Kinds: []int{game.SaveKind}, Tags: map[string][]string{"d": {d}}}, nil, 5*time.Second)
	var newest *nostr.Event
	for _, ev := range evs {
		if newest == nil || ev.CreatedAt > newest.CreatedAt {
			newest = ev
		}
	}
	if newest != nil && (cachedEv == nil || newest.ID != cachedEv.ID) &&
		(cacheErr != nil || cachedEv == nil || newest.CreatedAt > cachedEv.CreatedAt) {
		if save, err := decodeSaveEvent(newest, pubkey, d); err == nil {
			save.InternalID, save.InternalNpub = saveID, npub
			cacheSave(npub, saveID, save, newest)
			return save, nil
		}
	}
	if cacheErr != nil {
		return nil, cacheErr
	}
	return cached, nil
}

// SaveNote explains, for the saves screen, when relays hold a different
// copy of a character than the one this server will load.
type SaveNote struct {
	SaveID string `json:"save_id"`
	Note   string `json:"note"`
}

// syncRelaySaves makes sure every character the player has on relays (or, on
// the official server, in the ledger) has a disk cache to list, and notes
// mismatches. Called by the saves listing when relay saves are on.
func syncRelaySaves(npub string) []SaveNote {
	pubkey, err := tools.DecodeNpub(npub)
	if err != nil {
		return nil
	}
	evs := fetchEvents(pubkey, nostr.Filter{Authors: []string{pubkey}, Kinds: []int{game.SaveKind}}, nil, 5*time.Second)
	newestBySlot := map[string]*nostr.Event{}
	for _, ev := range evs {
		id := game.SaveIDFromDTag(tagValue(ev, "d"))
		if id == "" {
			continue
		}
		if cur := newestBySlot[id]; cur == nil || ev.CreatedAt > cur.CreatedAt {
			newestBySlot[id] = ev
		}
	}

	var notes []SaveNote
	if utils.AppConfig.Saves.Validation {
		for d, entry := range ledger.List(npub) {
			saveID := game.SaveIDFromDTag(d)
			if saveID == "" {
				continue
			}
			if _, err := os.Stat(session.GetSavePath(npub, saveID)); err != nil {
				relaySaveSource(npub, saveID) // fetch the accepted copy into the cache
			}
			if ev := newestBySlot[saveID]; ev != nil && ev.ID != entry.EventID {
				notes = append(notes, SaveNote{SaveID: saveID, Note: "A different copy of this character is on your relays (edited, from another game, or older). This server loads the last save it accepted."})
			}
		}
		return notes
	}
	for saveID := range newestBySlot {
		if _, err := os.Stat(session.GetSavePath(npub, saveID)); err != nil {
			relaySaveSource(npub, saveID)
		}
	}
	return notes
}

// InitRelaySaves switches loading over to relays when the config asks for it.
func InitRelaySaves() {
	if !utils.AppConfig.Saves.Relay {
		return
	}
	session.SaveSource = relaySaveSource
	mode := "test/modded (any signed save)"
	if utils.AppConfig.Saves.Validation {
		mode = "official (ledger-validated)"
	}
	log.Printf("✅ Relay saves on — %s, slots: %d (0 = unlimited)", mode, utils.AppConfig.Saves.MaxSlots)
}

// shortID shortens an event id for messages.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
