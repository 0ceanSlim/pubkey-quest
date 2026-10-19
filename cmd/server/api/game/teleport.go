package game

import (
	"encoding/json"
	"log"
	"net/http"

	"pubkey-quest/cmd/server/game/teleport"
	"pubkey-quest/cmd/server/social"
)

// ─── Teleportation (Homeward Stones + waystones) ────────────────────────────
//
// The rules live in game/teleport; this is the session-side guard (no fight,
// no POI walk, no party) and the two actions:
//
//	use_homeward_stone {item_id}   — to that stone's city's waystone
//	waystone_hop       {destination} — from the waystone you stand at
//
// Neither advances the clock, so processGameAction rolls no travel or
// encounter for them.

// teleportBlocked says why the character can't teleport right now ("" = fine).
func teleportBlocked(session *GameSession) string {
	switch {
	case session.ActiveCombat != nil:
		return "Not in the middle of a fight."
	case session.ActivePOI != nil:
		return "Not from in here. Finish what you're doing first."
	case session.PendingEncounter != nil:
		return "Not now: something on the road has your attention."
	case session.LockSession != nil:
		return "Not with your hands full of picks."
	}
	// A party would be left behind (the leader's moves carry followers along,
	// and they may have no stone or charge of their own).
	if p := social.PartyOf(session.Npub); p != nil && len(p.Members) > 1 {
		return "The stone carries only you. Leave your party first."
	}
	return ""
}

func teleportResponse(res *teleport.Result) *GameActionResponse {
	return &GameActionResponse{Success: true, Message: res.Message, Color: "purple", Data: map[string]interface{}{
		"teleported": true, "city": res.City, "discovered": res.Discovered, "music_unlocked": res.Music,
	}}
}

func handleUseHomewardStone(session *GameSession, params map[string]any) (*GameActionResponse, error) {
	if why := teleportBlocked(session); why != "" {
		return &GameActionResponse{Success: false, Message: why, Color: "yellow"}, nil
	}
	itemID, _ := params["item_id"].(string)
	res, err := teleport.UseStone(&session.SaveData, itemID)
	if err != nil {
		return &GameActionResponse{Success: false, Message: err.Error(), Color: "yellow"}, nil
	}
	log.Printf("🌀 %s used %s → %s", session.Npub, itemID, res.City)
	return teleportResponse(res), nil
}

func handleWaystoneHop(session *GameSession, params map[string]any) (*GameActionResponse, error) {
	if why := teleportBlocked(session); why != "" {
		return &GameActionResponse{Success: false, Message: why, Color: "yellow"}, nil
	}
	dest, _ := params["destination"].(string)
	res, err := teleport.Hop(&session.SaveData, dest)
	if err != nil {
		return &GameActionResponse{Success: false, Message: err.Error(), Color: "yellow"}, nil
	}
	log.Printf("🌀 %s hopped the waystones → %s", session.Npub, res.City)
	return teleportResponse(res), nil
}

// TeleportStatusHandler godoc
// @Summary      Homeward Stones and waystones for this character
// @Description  The stones they carry, both cooldowns, whether they stand at a waystone, and every
// @Description  waystone destination (woken or dormant), plus why teleporting is blocked right now.
// @Tags         Game
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save ID"
// @Success      200      {object}  map[string]interface{}
// @Router       /api/teleport/status [get]
func TeleportStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	npub, saveID := r.URL.Query().Get("npub"), r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		http.Error(w, "Missing query params: npub, save_id", http.StatusBadRequest)
		return
	}
	sess := getSessionAndValidate(w, npub, saveID)
	if sess == nil {
		return
	}
	save := sess.GetSaveData()
	cfg := teleport.Load()
	type stoneView struct {
		ItemID string `json:"item_id"`
		City   string `json:"city"`
		Name   string `json:"name"`
	}
	var stones []stoneView
	for _, id := range cfg.Chain {
		c := cfg.Cities[id]
		if teleport.Holds(save, c.Stone) {
			stones = append(stones, stoneView{ItemID: c.Stone, City: id, Name: c.Name})
		}
	}
	stoneWait := teleport.CooldownLeft(save, cfg.StoneCooldownEffect)
	hopWait := teleport.CooldownLeft(save, cfg.WaystoneCooldownEffect)
	data := map[string]interface{}{
		"stones":                 stones,
		"stone_cooldown":         stoneWait,
		"stone_cooldown_text":    waitText(stoneWait),
		"waystone_here":          teleport.WaystoneHere(save),
		"waystone_cooldown":      hopWait,
		"waystone_cooldown_text": waitText(hopWait),
		"destinations":           teleport.Destinations(save),
		"blocked":                teleportBlocked(sess),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": data})
}

func waitText(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	return teleport.FormatWait(minutes)
}
