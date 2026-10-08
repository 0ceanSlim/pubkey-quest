package api

import (
	"encoding/json"
	"log"
	"net/http"

	grainCache "github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/client/session"
)

// A player's relay list, so they can change where this game reads and writes
// without leaving for another client.
//
// Two layers, and the distinction matters in the UI:
//
//   - The NIP-65 list (kind 10002) is the player's published answer to "where
//     do you read and write?" — portable, and other clients honour it. Editing
//     it means publishing a new kind 10002 through /api/publish.
//   - Pinned relays are a local override: ignore the outbox model and use
//     exactly these. For a player who wants this game's saves on a specific set
//     of relays, that is the switch. It lives on their UserContext, affects only
//     them, and is published nowhere.

// RelayEntry is one relay and what the player uses it for.
type RelayEntry struct {
	URL   string `json:"url"`
	Read  bool   `json:"read"`
	Write bool   `json:"write"`
}

// RelayListResponse describes where a player's events currently go.
type RelayListResponse struct {
	Npub   string       `json:"npub"`
	Relays []RelayEntry `json:"relays"`
	// Pinned reports whether the player has overridden outbox routing.
	Pinned bool `json:"pinned"`
	// Connected are the relays actually held open for this player right now —
	// useful for showing which of their choices are reachable.
	Connected []string `json:"connected"`
}

// RelaysHandler godoc
// @Summary      Get the signed-in player's relays
// @Description  Returns the player's relay list with read/write roles, whether
// @Description  they have pinned relays instead of using the outbox model, and
// @Description  which relays are currently connected for them.
// @Tags         Profile
// @Produce      json
// @Success      200  {object}  RelayListResponse
// @Failure      401  {string}  string  "Not signed in"
// @Router       /relays [get]
func RelaysHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := session.SessionMgr.GetCurrentUser(r)
	if user == nil {
		http.Error(w, "Not signed in", http.StatusUnauthorized)
		return
	}

	writeJSON(w, relayState(user.PublicKey))
}

// relayState describes where a player's events currently go.
func relayState(pubKey string) RelayListResponse {
	resp := RelayListResponse{Relays: []RelayEntry{}, Connected: []string{}}
	if npub, err := tools.EncodePubkey(pubKey); err == nil {
		resp.Npub = npub
	}

	// Grain caches the player's relays with their read/write permissions, warmed
	// from their mailboxes at login.
	configured, err := grainCache.GetUserClientRelaysWithPermissions(pubKey)
	if err != nil {
		log.Printf("⚠️ No cached relays for %s...: %v", shortKey(pubKey), err)
	}
	for _, c := range configured {
		resp.Relays = append(resp.Relays, RelayEntry{URL: c.URL, Read: c.Read, Write: c.Write})
	}

	if uc := connection.UserFor(pubKey); uc != nil {
		resp.Pinned = uc.FixedRelaysEnabled()
		resp.Connected = uc.HeldRelays()
	}
	return resp
}

// PinnedRelaysRequest turns the local override on or off.
type PinnedRelaysRequest struct {
	Enabled bool `json:"enabled"`
}

// PinnedRelaysHandler godoc
// @Summary      Pin this player to their own relay list
// @Description  Turns off outbox routing for the signed-in player and uses
// @Description  exactly the relays on their list instead. Local to that player
// @Description  and published nowhere; clearing it restores outbox routing.
// @Tags         Profile
// @Accept       json
// @Produce      json
// @Param        request  body      PinnedRelaysRequest  true  "Whether to pin"
// @Success      200      {object}  RelayListResponse
// @Failure      401      {string}  string  "Not signed in"
// @Router       /relays/pinned [post]
func PinnedRelaysHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := session.SessionMgr.GetCurrentUser(r)
	if user == nil {
		http.Error(w, "Not signed in", http.StatusUnauthorized)
		return
	}

	var req PinnedRelaysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	uc := connection.UserFor(user.PublicKey)
	if uc == nil {
		http.Error(w, "Relay client not ready — try again in a moment", http.StatusInternalServerError)
		return
	}

	if !req.Enabled {
		uc.ClearFixedRelays()
		log.Printf("📡 %s... back to outbox routing", shortKey(user.PublicKey))
		writeJSON(w, relayState(user.PublicKey))
		return
	}

	// Pin to the player's own list, split by the role each relay carries. A
	// relay with neither role is no use to pin to, so it is left out.
	configured, err := grainCache.GetUserClientRelaysWithPermissions(user.PublicKey)
	if err != nil || len(configured) == 0 {
		http.Error(w, "Add some relays before pinning to them", http.StatusBadRequest)
		return
	}

	var read, write []string
	for _, c := range configured {
		if c.Read {
			read = append(read, c.URL)
		}
		if c.Write {
			write = append(write, c.URL)
		}
	}
	if len(read) == 0 && len(write) == 0 {
		http.Error(w, "None of your relays are marked read or write", http.StatusBadRequest)
		return
	}

	uc.PinFixedRelays(read, write)
	log.Printf("📌 %s... pinned to %d read / %d write relays", shortKey(user.PublicKey), len(read), len(write))
	writeJSON(w, relayState(user.PublicKey))
}
