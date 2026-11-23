package auth

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// Amber (NIP-55) answers by opening our callback page in the phone's default
// browser, which needn't be the browser, or the installed app, the player is
// playing in: browser storage doesn't cross that gap. So the game puts a
// one-time code of its own in the callback URL
// (/api/auth/amber-callback/<code>), the page posts Amber's answer here under
// it, and the game tab collects it. The answer is a pubkey (signing in, which
// still needs a signed proof) or a signed event (verifiable, and published
// anyway), so holding it a moment is safe.

var amberCode = regexp.MustCompile(`^[0-9a-f]{32}$`)

const (
	amberTTL      = 5 * time.Minute
	amberMaxBody  = 64 << 10
	amberMaxHeld  = 10000
	amberSweepAge = time.Minute
)

type amberAnswer struct {
	Event string `json:"event,omitempty"`
	Error string `json:"error,omitempty"`
	at    time.Time
}

var (
	amberMu      sync.Mutex
	amberHeld    = map[string]amberAnswer{}
	amberSweptAt time.Time
)

// HandleAmberResult takes Amber's answer from the callback page (POST
// {code, event, error}) and hands it to the game tab, once (GET ?code=).
func (auth *AuthHandler) HandleAmberResult(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var in struct {
			Code  string `json:"code"`
			Event string `json:"event"`
			Error string `json:"error"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, amberMaxBody)).Decode(&in); err != nil ||
			!amberCode.MatchString(in.Code) || (in.Event == "" && in.Error == "") {
			http.Error(w, "bad answer", http.StatusBadRequest)
			return
		}
		if !holdAmber(in.Code, amberAnswer{Event: in.Event, Error: in.Error, at: time.Now()}) {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodGet:
		code := r.URL.Query().Get("code")
		if !amberCode.MatchString(code) {
			http.Error(w, "bad code", http.StatusBadRequest)
			return
		}
		a, ok := takeAmber(code)
		if !ok {
			w.WriteHeader(http.StatusNoContent) // not yet
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(a)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// holdAmber keeps an answer for its code (the newest wins); false when the
// store is full of fresh answers.
func holdAmber(code string, a amberAnswer) bool {
	amberMu.Lock()
	defer amberMu.Unlock()
	if now := time.Now(); now.Sub(amberSweptAt) > amberSweepAge || len(amberHeld) >= amberMaxHeld {
		for c, held := range amberHeld {
			if now.Sub(held.at) > amberTTL {
				delete(amberHeld, c)
			}
		}
		amberSweptAt = now
	}
	if _, exists := amberHeld[code]; !exists && len(amberHeld) >= amberMaxHeld {
		return false
	}
	amberHeld[code] = a
	return true
}

// takeAmber hands over an answer once; an old one is gone.
func takeAmber(code string) (amberAnswer, bool) {
	amberMu.Lock()
	defer amberMu.Unlock()
	a, ok := amberHeld[code]
	delete(amberHeld, code)
	if !ok || time.Since(a.at) > amberTTL {
		return amberAnswer{}, false
	}
	return a, true
}
