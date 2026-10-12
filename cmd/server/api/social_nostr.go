package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/client/session"
	nostr "github.com/0ceanslim/grain/server/types"

	"pubkey-quest/cmd/server/api/game"
)

// ─── Friends, follows, mutes and whispers over Nostr (social-plan §5, §7, §8) ─
//
// Friends are mutual follows: people in your contact list (kind 3) whose own
// contact list has you. Follow/unfollow and mute edit your real Nostr lists —
// the browser signs the new list, and the server only accepts it if it differs
// from your newest one by exactly one entry, so it can never clobber follows
// you made in other clients. Whispers are NIP-17 DMs sealed and gift-wrapped in
// the browser; the server just relays the wraps to the recipient's DM inbox and
// fetches yours.

const (
	friendsTTL   = 10 * time.Minute
	statusTTL    = 5 * time.Minute
	authorsChunk = 150
)

// FriendView is one mutual follow.
// swagger:model FriendView
type FriendView struct {
	Npub        string           `json:"npub"`
	Name        string           `json:"name"`
	Picture     string           `json:"picture,omitempty"`
	InGame      *game.PlayerView `json:"in_game,omitempty"`
	LastEventAt int64            `json:"last_event_at,omitempty"` // unix seconds of their latest Nostr event
	Client      string           `json:"client,omitempty"`        // that event's NIP-89 client tag
}

// FriendsResponse lists your friends.
// swagger:model FriendsResponse
type FriendsResponse struct {
	Success bool         `json:"success"`
	Friends []FriendView `json:"friends"`
	Follows []string     `json:"follows"` // everyone you follow (npubs), for the Follow / Unfollow menu
}

type friendsEntry struct {
	at      time.Time
	follows []string // hex
	friends []string // hex
}

type statusEntry struct {
	at     time.Time
	last   int64
	client string
}

var (
	friendsMu    sync.Mutex
	friendsCache = map[string]friendsEntry{}
	statusCache  = map[string]statusEntry{}
)

func coreClient() *core.Client { return connection.GetCoreClient() }

// newest returns the event with the highest created_at.
func newest(evs []*nostr.Event) *nostr.Event {
	var best *nostr.Event
	for _, e := range evs {
		if e != nil && (best == nil || e.CreatedAt > best.CreatedAt) {
			best = e
		}
	}
	return best
}

// latestList reads a user's newest replaceable list of the given kind from
// their own relays plus the index relays (newest wins, not first EOSE).
func latestList(ctx context.Context, pubkey string, kind int) *nostr.Event {
	c := coreClient()
	if c == nil {
		return nil
	}
	f := nostr.Filter{Authors: []string{pubkey}, Kinds: []int{kind}}
	return newest(c.QueryEvents(ctx, f, c.OwnListRelays(pubkey), core.WithTimeout(6*time.Second)))
}

// pTags returns the hex pubkeys a list tags.
func pTags(e *nostr.Event) []string {
	if e == nil {
		return nil
	}
	var out []string
	for _, t := range e.Tags {
		if len(t) >= 2 && t[0] == "p" && len(t[1]) == 64 {
			out = append(out, t[1])
		}
	}
	return out
}

// loadFriends works out follows and mutual follows for a user (cached).
func loadFriends(ctx context.Context, me string, fresh bool) friendsEntry {
	friendsMu.Lock()
	if e, ok := friendsCache[me]; ok && !fresh && time.Since(e.at) < friendsTTL {
		friendsMu.Unlock()
		return e
	}
	friendsMu.Unlock()

	entry := friendsEntry{at: time.Now()}
	entry.follows = pTags(latestList(ctx, me, 3))
	c := coreClient()
	if c != nil && len(entry.follows) > 0 {
		// Each followee's own contact list: do they follow me back?
		relays := c.AppRelays(core.RoleIndexer)
		relays = append(relays, c.OwnListRelays(me)...)
		latestBy := map[string]*nostr.Event{}
		for i := 0; i < len(entry.follows); i += authorsChunk {
			end := i + authorsChunk
			if end > len(entry.follows) {
				end = len(entry.follows)
			}
			f := nostr.Filter{Authors: entry.follows[i:end], Kinds: []int{3}}
			for _, e := range c.QueryEvents(ctx, f, relays, core.WithTimeout(8*time.Second)) {
				if cur := latestBy[e.PubKey]; cur == nil || e.CreatedAt > cur.CreatedAt {
					latestBy[e.PubKey] = e
				}
			}
		}
		for _, pk := range entry.follows {
			for _, t := range pTags(latestBy[pk]) {
				if t == me {
					entry.friends = append(entry.friends, pk)
					break
				}
			}
		}
	}
	friendsMu.Lock()
	friendsCache[me] = entry
	friendsMu.Unlock()
	return entry
}

// lastSeen finds friends' latest Nostr events (and the client that wrote
// them), for "active on Primal · 12 min ago".
func lastSeen(ctx context.Context, pubkeys []string) map[string]statusEntry {
	out := map[string]statusEntry{}
	var need []string
	friendsMu.Lock()
	for _, pk := range pubkeys {
		if s, ok := statusCache[pk]; ok && time.Since(s.at) < statusTTL {
			out[pk] = s
		} else {
			need = append(need, pk)
		}
	}
	friendsMu.Unlock()
	c := coreClient()
	if c == nil || len(need) == 0 {
		return out
	}
	since := time.Now().Add(-30 * 24 * time.Hour)
	relays := c.AppRelays(core.RoleIndexer)
	for i := 0; i < len(need); i += authorsChunk {
		end := i + authorsChunk
		if end > len(need) {
			end = len(need)
		}
		f := nostr.Filter{Authors: need[i:end], Since: &since}
		for _, e := range c.QueryEvents(ctx, f, relays, core.WithTimeout(6*time.Second), core.WithLimit(1000)) {
			cur := out[e.PubKey]
			if e.CreatedAt <= cur.last {
				continue
			}
			client := ""
			for _, t := range e.Tags {
				if len(t) >= 2 && t[0] == "client" {
					client = t[1]
				}
			}
			out[e.PubKey] = statusEntry{at: time.Now(), last: e.CreatedAt, client: client}
		}
	}
	friendsMu.Lock()
	for _, pk := range need {
		s := out[pk]
		s.at = time.Now()
		statusCache[pk] = s
	}
	friendsMu.Unlock()
	return out
}

// FriendsHandler godoc
// @Summary      Your friends (mutual follows) and their status
// @Description  In game: where they are and the clock offset. Otherwise: their latest Nostr event's time and client.
// @Tags         Social
// @Produce      json
// @Param        npub     query     string  true   "Your npub"
// @Param        save_id  query     string  true   "Your save (for clock offsets)"
// @Param        refresh  query     bool    false  "Skip the cache"
// @Success      200      {object}  FriendsResponse
// @Router       /api/friends [get]
func FriendsHandler(w http.ResponseWriter, r *http.Request) {
	user := session.SessionMgr.GetCurrentUser(r)
	if user == nil {
		http.Error(w, "Not signed in", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	me := user.PublicKey
	entry := loadFriends(ctx, me, r.URL.Query().Get("refresh") == "true")
	status := lastSeen(ctx, entry.friends)
	viewerNpub, saveID := r.URL.Query().Get("npub"), r.URL.Query().Get("save_id")

	resp := FriendsResponse{Success: true, Friends: []FriendView{}, Follows: []string{}}
	for _, pk := range entry.follows {
		if n, err := tools.EncodePubkey(pk); err == nil {
			resp.Follows = append(resp.Follows, n)
		}
	}
	for _, pk := range entry.friends {
		npub, err := tools.EncodePubkey(pk)
		if err != nil {
			continue
		}
		fv := FriendView{Npub: npub, LastEventAt: status[pk].last, Client: status[pk].client}
		prof := fetchProfile(pk)
		fv.Name = prof.Profile.DisplayName
		if fv.Name == "" {
			fv.Name = prof.Profile.Name
		}
		if fv.Name == "" {
			fv.Name = npub[:12] + "…"
		}
		fv.Picture = prof.Profile.Picture
		fv.InGame = game.FriendInGame(viewerNpub, saveID, npub)
		resp.Friends = append(resp.Friends, fv)
	}
	sort.SliceStable(resp.Friends, func(i, j int) bool {
		a, b := resp.Friends[i], resp.Friends[j]
		if (a.InGame != nil) != (b.InGame != nil) {
			return a.InGame != nil
		}
		return a.LastEventAt > b.LastEventAt
	})
	writeJSON(w, resp)
}

// ─── Editing your own lists (kind 3 contacts, kind 10000 mutes) ──────────────

// ListResponse carries your newest list event (raw), for editing.
// swagger:model ListResponse
type ListResponse struct {
	Success bool         `json:"success"`
	Event   *nostr.Event `json:"event"`
}

// listHandler serves GET (your newest list) and POST (publish an edited one)
// for a list kind.
func listHandler(kind int, label string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := session.SessionMgr.GetCurrentUser(r)
		if user == nil {
			http.Error(w, "Not signed in", http.StatusUnauthorized)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		me := user.PublicKey
		current := latestList(ctx, me, kind)

		if r.Method == http.MethodGet {
			writeJSON(w, ListResponse{Success: true, Event: current})
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Event *nostr.Event `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Event == nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Missing event"})
			return
		}
		ev := req.Event
		switch {
		case ev.Kind != kind:
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Wrong kind"})
			return
		case ev.PubKey != me:
			writeJSONStatus(w, http.StatusForbidden, map[string]any{"success": false, "error": "That list was signed by a different key"})
			return
		case !core.VerifyEventSignature(ev):
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Bad signature"})
			return
		}
		if err := oneEntryChange(current, ev); err != nil {
			writeJSONStatus(w, http.StatusConflict, map[string]any{"success": false, "error": err.Error()})
			return
		}
		uc := connection.UserFor(me)
		if uc == nil {
			writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"success": false, "error": "Relay client not ready — try again in a moment"})
			return
		}
		relays := uc.RoutePublish(ev)
		results, err := uc.PublishEvent(ctx, ev, relays)
		accepted := 0
		for _, res := range results {
			if res.Accepted { // the relay's OK, not just "sent"
				accepted++
			}
		}
		if err != nil || accepted == 0 {
			writeJSONStatus(w, http.StatusBadGateway, map[string]any{"success": false, "error": fmt.Sprintf("No relay took your %s", label)})
			return
		}
		friendsMu.Lock()
		delete(friendsCache, me)
		friendsMu.Unlock()
		log.Printf("✅ Published %s for %s... to %d/%d relays", label, shortKey(me), accepted, len(relays))
		writeJSON(w, map[string]any{"success": true, "event_id": ev.ID})
	}
}

// oneEntryChange refuses an edited list unless it is newer than the current
// one and differs by exactly one public p tag (added or removed), with every
// other tag and the content left as they were.
func oneEntryChange(current, next *nostr.Event) error {
	if current == nil {
		if len(pTags(next)) > 1 {
			return fmt.Errorf("your list couldn't be read — refusing to replace it")
		}
		return nil
	}
	if next.CreatedAt <= current.CreatedAt {
		return fmt.Errorf("your list changed elsewhere — reload and try again")
	}
	if next.Content != current.Content {
		return fmt.Errorf("the list's private part changed — refusing")
	}
	key := func(t []string) string { return strings.Join(t, "\x00") }
	count := func(e *nostr.Event) map[string]int {
		m := map[string]int{}
		for _, t := range e.Tags {
			m[key(t)]++
		}
		return m
	}
	a, b := count(current), count(next)
	diffs := 0
	var changed string
	for k, n := range a {
		if b[k] != n {
			diffs += abs(b[k] - n)
			changed = k
		}
	}
	for k, n := range b {
		if _, ok := a[k]; !ok {
			diffs += n
			changed = k
		}
	}
	if diffs != 1 || !strings.HasPrefix(changed, "p\x00") {
		return fmt.Errorf("a list update may add or remove exactly one person")
	}
	return nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ContactsHandler godoc
// @Summary      Your newest contact list (GET) / publish one edited by exactly one follow (POST)
// @Tags         Social
// @Router       /api/friends/contacts [get]
var ContactsHandler = listHandler(3, "contact list")

// MutesHandler godoc
// @Summary      Your newest mute list (GET) / publish one edited by exactly one entry (POST)
// @Tags         Social
// @Router       /api/mutes [get]
var MutesHandler = listHandler(10000, "mute list")

// ─── Whispers: NIP-17 gift wraps ─────────────────────────────────────────────

var (
	dmMu     sync.Mutex
	dmSentAt = map[string][]time.Time{}
)

// DMSendHandler godoc
// @Summary      Deliver whisper gift wraps
// @Description  Takes the browser-built kind 1059 wraps (one for the recipient, one for your own copy) and
// @Description  publishes each to its recipient's DM inbox relays (kind 10050, else their read relays).
// @Tags         Social
// @Accept       json
// @Produce      json
// @Router       /api/dm/send [post]
func DMSendHandler(w http.ResponseWriter, r *http.Request) {
	user := session.SessionMgr.GetCurrentUser(r)
	if user == nil {
		http.Error(w, "Not signed in", http.StatusUnauthorized)
		return
	}
	var req struct {
		Wraps []*nostr.Event `json:"wraps"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil || len(req.Wraps) == 0 || len(req.Wraps) > 2 {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Send one or two wraps"})
		return
	}
	// 10 whispers a minute.
	dmMu.Lock()
	now := time.Now()
	recent := dmSentAt[user.PublicKey][:0]
	for _, t := range dmSentAt[user.PublicKey] {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 10 {
		dmSentAt[user.PublicKey] = recent
		dmMu.Unlock()
		writeJSONStatus(w, http.StatusTooManyRequests, map[string]any{"success": false, "error": "Slow down."})
		return
	}
	dmSentAt[user.PublicKey] = append(recent, now)
	dmMu.Unlock()

	c := coreClient()
	uc := connection.UserFor(user.PublicKey)
	if c == nil || uc == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"success": false, "error": "Relay client not ready"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	delivered := 0
	for _, wrap := range req.Wraps {
		var to []string
		for _, t := range wrap.Tags {
			if len(t) >= 2 && t[0] == "p" {
				to = append(to, t[1])
			}
		}
		if wrap.Kind != 1059 || len(to) != 1 || len(wrap.Content) > 64<<10 || !core.VerifyEventSignature(wrap) {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Malformed wrap"})
			return
		}
		ur := c.ResolveRelays(to[0])
		relays := ur.DMInbox
		if len(relays) == 0 {
			relays = ur.Inbox
		}
		if len(relays) == 0 {
			relays = c.AppRelays(core.RoleIndexer)
		}
		results, _ := uc.PublishEvent(ctx, wrap, relays)
		for _, res := range results {
			if res.Accepted { // the relay's OK, not just "sent"
				delivered++
				break
			}
		}
	}
	writeJSON(w, map[string]any{"success": delivered > 0, "delivered": delivered})
}

// DMInboxHandler godoc
// @Summary      Your incoming whisper wraps
// @Description  Raw kind 1059 events tagged to you on your DM inbox relays, for the browser to unwrap. NIP-59
// @Description  randomises wrap timestamps up to two days back, so `since` is widened by two days.
// @Tags         Social
// @Produce      json
// @Param        since  query  int  false  "Unix seconds"
// @Router       /api/dm/inbox [get]
func DMInboxHandler(w http.ResponseWriter, r *http.Request) {
	user := session.SessionMgr.GetCurrentUser(r)
	if user == nil {
		http.Error(w, "Not signed in", http.StatusUnauthorized)
		return
	}
	c := coreClient()
	if c == nil {
		writeJSON(w, map[string]any{"success": true, "wraps": []any{}})
		return
	}
	var since time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		var n int64
		fmt.Sscan(s, &n)
		since = time.Unix(n, 0).Add(-48 * time.Hour)
	} else {
		since = time.Now().Add(-72 * time.Hour)
	}
	ur := c.ResolveRelays(user.PublicKey)
	relays := ur.DMInbox
	if len(relays) == 0 {
		relays = ur.Inbox
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	f := nostr.Filter{Kinds: []int{1059}, Tags: map[string][]string{"p": {user.PublicKey}}, Since: &since}
	// Read over the player's own connections, so relays they've answered a
	// NIP-42 challenge on (see DMAuthHandler) serve their DMs.
	var wraps []*nostr.Event
	if uc := connection.UserFor(user.PublicKey); uc != nil {
		wraps = uc.QueryEvents(ctx, f, relays, core.WithTimeout(8*time.Second), core.WithLimit(200))
	} else {
		wraps = c.QueryEvents(ctx, f, relays, core.WithTimeout(8*time.Second), core.WithLimit(200))
	}
	if wraps == nil {
		wraps = []*nostr.Event{}
	}
	writeJSON(w, map[string]any{"success": true, "wraps": wraps})
}

// ─── NIP-42: logging in to DM relays (social-plan S5b) ───────────────────────

// AuthChallengeView is a DM relay asking you to prove your key.
// swagger:model AuthChallengeView
type AuthChallengeView struct {
	Relay     string `json:"relay"`
	Challenge string `json:"challenge"`
}

// DMAuthHandler godoc
// @Summary      NIP-42 for your DM inbox relays
// @Description  GET lists your DM relays that challenge you and aren't answered yet (relay + challenge).
// @Description  POST {relay, event} answers one with a kind 22242 event signed in the browser; from then on
// @Description  your requests to that relay use the authenticated connection.
// @Tags         Social
// @Router       /api/dm/auth [get]
func DMAuthHandler(w http.ResponseWriter, r *http.Request) {
	user := session.SessionMgr.GetCurrentUser(r)
	if user == nil {
		http.Error(w, "Not signed in", http.StatusUnauthorized)
		return
	}
	uc := connection.UserFor(user.PublicKey)
	c := coreClient()
	if uc == nil || c == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"success": false, "error": "Relay client not ready"})
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Relay string       `json:"relay"`
			Event *nostr.Event `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Event == nil || req.Relay == "" {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Missing relay or event"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := uc.Authenticate(ctx, req.Relay, req.Event); err != nil {
			writeJSONStatus(w, http.StatusBadGateway, map[string]any{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"success": true})
		return
	}

	ur := c.ResolveRelays(user.PublicKey)
	relays := ur.DMInbox
	if len(relays) == 0 {
		relays = ur.Inbox
	}
	out := []AuthChallengeView{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, relay := range relays {
		if uc.IsAuthenticated(relay) {
			continue
		}
		wg.Add(1)
		go func(relay string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
			defer cancel()
			// Relays that don't challenge don't need AUTH: skip them.
			if ch, err := uc.AuthChallenge(ctx, relay); err == nil && ch != "" {
				mu.Lock()
				out = append(out, AuthChallengeView{Relay: relay, Challenge: ch})
				mu.Unlock()
			}
		}(relay)
	}
	wg.Wait()
	writeJSON(w, map[string]any{"success": true, "challenges": out})
}
