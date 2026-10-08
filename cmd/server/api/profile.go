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

// ProfileResponse represents the response from the profile endpoint.
//
// Profile is a convenience view of the fields the game cares about. Fields and
// Tags are the event as it actually is — every content key and every tag,
// including ones this client knows nothing about. An editor MUST merge onto
// those rather than onto Profile: kind 0 is replaceable, so publishing a subset
// deletes the rest. NIP-39 identity claims live in tags, and losing them
// un-verifies somebody's GitHub or Twitter.
//
// swagger:model ProfileResponse
type ProfileResponse struct {
	Npub      string          `json:"npub"`
	Pubkey    string          `json:"pubkey"`
	Profile   ProfileMetadata `json:"profile"`
	Fields    map[string]any  `json:"fields"`
	Tags      [][]string      `json:"tags"`
	CreatedAt int64           `json:"created_at,omitempty"`
	Found     bool            `json:"found"`
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
	resp := fetchProfile(pubKey)
	resp.Npub = npub
	resp.Pubkey = pubKey
	writeJSON(w, resp)
}

// fetchProfile returns a pubkey's kind 0, both as the typed view the game reads
// and as the raw fields and tags an editor needs in order not to destroy the
// parts it does not understand.
func fetchProfile(pubKey string) ProfileResponse {
	resp := ProfileResponse{Fields: map[string]any{}, Tags: [][]string{}}

	event, _, err := data.GetUserDataForSession(pubKey)
	if err != nil {
		log.Printf("⚠️ No profile for %s...: %v", shortKey(pubKey), err)
		return resp
	}
	if event == nil || event.Content == "" {
		return resp
	}

	resp.CreatedAt = event.CreatedAt
	if event.Tags != nil {
		resp.Tags = event.Tags
	}

	// A kind 0 whose content isn't the object we expect is the author's
	// business, not an error of ours — report it as absent.
	if err := json.Unmarshal([]byte(event.Content), &resp.Fields); err != nil {
		log.Printf("⚠️ Unparseable profile content for %s...: %v", shortKey(pubKey), err)
		return ProfileResponse{Fields: map[string]any{}, Tags: [][]string{}}
	}
	if err := json.Unmarshal([]byte(event.Content), &resp.Profile); err != nil {
		log.Printf("⚠️ Profile content has unexpected field types for %s...: %v", shortKey(pubKey), err)
	}
	resp.Found = true
	return resp
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
