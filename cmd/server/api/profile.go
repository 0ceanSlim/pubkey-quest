package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/client/data"
)

// Profiles come from Nostr, through Grain: data.GetUserDataForSession answers
// from Grain's cache when it can and otherwise fetches over the outbox model —
// the user's own relays, resolved from their NIP-65 list — rather than a handful
// of relays we guessed at. Login already warms that cache in the background
// (session.CreateUserSession), so the common case is a cache hit.
//
// This endpoint is public on purpose: kind 0 is public data, and the game shows
// other players' names and pictures.

// ProfileMetadata represents a Nostr user profile
// swagger:model ProfileMetadata
type ProfileMetadata struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	About       string `json:"about"`
	Picture     string `json:"picture"`
	Banner      string `json:"banner"`
	Nip05       string `json:"nip05"`
	Lud16       string `json:"lud16"`
	Website     string `json:"website"`
}

// ProfileResponse represents the response from the profile endpoint
// swagger:model ProfileResponse
type ProfileResponse struct {
	Npub    string          `json:"npub"`
	Pubkey  string          `json:"pubkey"`
	Profile ProfileMetadata `json:"profile"`
	Found   bool            `json:"found"`
}

// ProfileHandler godoc
// @Summary      Get Nostr profile
// @Description  Fetch Nostr profile metadata (kind 0) for a given npub, via the
// @Description  outbox model. A player with no published profile is not an
// @Description  error: the response carries empty fields and found=false.
// @Tags         Profile
// @Accept       json
// @Produce      json
// @Param        npub  query     string  true  "Nostr public key (npub format)"
// @Success      200   {object}  ProfileResponse
// @Failure      400   {string}  string  "Missing or invalid npub"
// @Router       /profile [get]
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	npub := r.URL.Query().Get("npub")
	if npub == "" {
		http.Error(w, "Missing npub parameter", http.StatusBadRequest)
		return
	}

	pubKey, err := tools.DecodeNpub(npub)
	if err != nil {
		http.Error(w, "Invalid npub", http.StatusBadRequest)
		return
	}

	// A missing or unreachable profile is a normal state, not a failure: plenty
	// of players arrive having never published a kind 0, and the UI falls back
	// to a shortened npub. Answering 404/500 here only made the client log
	// errors over something ordinary.
	profile, found := fetchProfileMetadata(pubKey)

	writeJSON(w, ProfileResponse{
		Npub:    npub,
		Pubkey:  pubKey,
		Profile: profile,
		Found:   found,
	})
}

// fetchProfileMetadata returns the parsed kind 0 for a pubkey, and whether one
// was actually found.
func fetchProfileMetadata(pubKey string) (ProfileMetadata, bool) {
	var profile ProfileMetadata

	event, _, err := data.GetUserDataForSession(pubKey)
	if err != nil {
		log.Printf("⚠️ No profile for %s...: %v", shortKey(pubKey), err)
		return profile, false
	}
	if event == nil || event.Content == "" {
		return profile, false
	}
	// A kind 0 whose content isn't the object we expect is the author's
	// business, not an error of ours — report it as absent.
	if err := json.Unmarshal([]byte(event.Content), &profile); err != nil {
		log.Printf("⚠️ Unparseable profile content for %s...: %v", shortKey(pubKey), err)
		return ProfileMetadata{}, false
	}
	return profile, true
}

// shortKey abbreviates a pubkey for logging.
func shortKey(pubKey string) string {
	if len(pubKey) <= 8 {
		return pubKey
	}
	return pubKey[:8]
}

// writeJSON writes v as a JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("❌ Failed to encode response: %v", err)
	}
}
