package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	grainCache "github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	nostr "github.com/0ceanslim/grain/server/types"
)

// Publishing a player's own Nostr events.
//
// The browser signs (mill holds the key; the server never does), and the server
// verifies and routes. Routing happens on the player's own UserContext, so the
// event goes to *their* write relays — resolved from their NIP-65 list, or the
// relays they pinned — over their own NIP-42-authenticated connections where
// they have them. Nobody else's relay leases or auth state are touched.
//
// Mirrors Grain's client/api/profile.go PublishSignedHandler, including its
// cache invalidation: a republished relay list has to take effect immediately,
// or the next lookup keeps routing to the relays it replaced.

// publishableKinds are the event kinds a player may publish through the game.
// Deliberately a whitelist: this endpoint exists to let somebody edit their
// identity, not to turn the game into a general-purpose publish relay.
var publishableKinds = map[int]string{
	0:     "profile",
	10002: "relay list",
}

// PublishRequest carries a browser-signed event.
type PublishRequest struct {
	Event *nostr.Event `json:"event"`
}

// PublishResponse reports where an event landed.
type PublishResponse struct {
	Success  bool     `json:"success"`
	EventID  string   `json:"event_id,omitempty"`
	Relays   []string `json:"relays,omitempty"`
	Accepted int      `json:"accepted,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// PublishHandler godoc
// @Summary      Publish a signed Nostr event
// @Description  Accepts an event signed in the browser, verifies it belongs to
// @Description  the logged-in player, and publishes it to that player's own
// @Description  relays. Only profile (kind 0) and relay lists (kind 10002) are
// @Description  accepted.
// @Tags         Profile
// @Accept       json
// @Produce      json
// @Param        request  body      PublishRequest  true  "Signed event"
// @Success      200      {object}  PublishResponse
// @Failure      400      {object}  PublishResponse  "Bad event or unsupported kind"
// @Failure      401      {object}  PublishResponse  "Not signed in"
// @Failure      403      {object}  PublishResponse  "Event belongs to another key"
// @Router       /publish [post]
func PublishHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := session.SessionMgr.GetCurrentUser(r)
	if user == nil {
		writePublishError(w, http.StatusUnauthorized, "You need to be signed in to publish")
		return
	}

	var req PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Event == nil {
		writePublishError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	label, ok := publishableKinds[req.Event.Kind]
	if !ok {
		writePublishError(w, http.StatusBadRequest, fmt.Sprintf("Kind %d can't be published here", req.Event.Kind))
		return
	}
	// The signature proves who signed it; this proves it is the person asking.
	if req.Event.PubKey != user.PublicKey {
		writePublishError(w, http.StatusForbidden, "That event was signed by a different key")
		return
	}
	if !core.VerifyEventSignature(req.Event) {
		writePublishError(w, http.StatusBadRequest, "Event signature is invalid")
		return
	}

	uc := connection.UserFor(user.PublicKey)
	if uc == nil {
		writePublishError(w, http.StatusInternalServerError, "Relay client not ready — try again in a moment")
		return
	}

	relays := uc.RoutePublish(req.Event)
	results, err := uc.PublishEvent(r.Context(), req.Event, relays)
	if err != nil {
		log.Printf("❌ Failed to publish %s for %s...: %v", label, shortKey(user.PublicKey), err)
		writeJSONStatus(w, http.StatusInternalServerError, PublishResponse{
			Error:  fmt.Sprintf("Could not publish your %s: %v", label, err),
			Relays: relays,
		})
		return
	}

	accepted := 0
	for _, res := range results {
		if res.Success {
			accepted++
		}
	}

	// Our own reads must see the change straight away: the profile cache would
	// otherwise keep serving the old kind 0, and a stale relay list would keep
	// routing to the relays it just replaced.
	invalidateAfterPublish(uc, req.Event)

	log.Printf("✅ Published %s for %s... to %d/%d relays", label, shortKey(user.PublicKey), accepted, len(relays))
	writeJSON(w, PublishResponse{
		Success:  accepted > 0,
		EventID:  req.Event.ID,
		Relays:   relays,
		Accepted: accepted,
	})
}

// invalidateAfterPublish drops whatever cached view the event supersedes.
func invalidateAfterPublish(uc *core.UserContext, ev *nostr.Event) {
	switch ev.Kind {
	case 0:
		grainCache.ClearUserData(ev.PubKey)
	case 10002:
		uc.Client().InvalidateUserRelays(ev.PubKey)
		uc.Client().InvalidateUserRelayLists(ev.PubKey)
	}
}

func writePublishError(w http.ResponseWriter, status int, msg string) {
	writeJSONStatus(w, status, PublishResponse{Error: msg})
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("❌ Failed to encode response: %v", err)
	}
}
