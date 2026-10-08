package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
)

// ─── Proof of key ownership at login (M5.6 G1) ───────────────────────────────
//
// The browser holds the key (mill); the server only ever sees a pubkey. Without a
// proof, anyone could POST someone else's pubkey to /api/auth/login and play as
// them. So login is a challenge–response:
//
//  1. GET /api/auth/challenge → a random single-use nonce (5-minute lifetime).
//  2. The browser signs a NIP-98 HTTP-auth event (kind 27235) for
//     POST /api/auth/login carrying the nonce in a "challenge" tag.
//  3. HandleLogin verifies the signature, that the event's pubkey is the one
//     logging in, that it is fresh, and burns the nonce.

const (
	// LoginProofKind is the NIP-98 HTTP Auth kind; signers know to show it as a
	// login request rather than a note.
	LoginProofKind = 27235

	challengeTTL = 5 * time.Minute
	// proofMaxSkew bounds how far the proof's created_at may sit from server time.
	proofMaxSkew = 5 * time.Minute
)

// challengeStore holds issued, unspent login nonces.
type challengeStore struct {
	mu      sync.Mutex
	pending map[string]time.Time // nonce → expiry
}

var challenges = &challengeStore{pending: map[string]time.Time{}}

// issue mints a new nonce and sweeps expired ones (cheap: the map stays tiny).
func (s *challengeStore) issue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(buf)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	for n, exp := range s.pending {
		if now.After(exp) {
			delete(s.pending, n)
		}
	}
	s.pending[nonce] = now.Add(challengeTTL)
	return nonce, nil
}

// consume spends a nonce, reporting whether it was issued and still live.
func (s *challengeStore) consume(nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.pending[nonce]
	if !ok {
		return false
	}
	delete(s.pending, nonce)
	return time.Now().Before(exp)
}

// ChallengeResponse is returned by GET /api/auth/challenge.
type ChallengeResponse struct {
	Success   bool   `json:"success"`
	Challenge string `json:"challenge"`
	Kind      int    `json:"kind"`
	ExpiresIn int    `json:"expires_in"` // seconds
}

// HandleChallenge issues a single-use login nonce.
func (auth *AuthHandler) HandleChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	nonce, err := challenges.issue()
	if err != nil {
		auth.sendErrorResponse(w, "Could not create a login challenge", http.StatusInternalServerError)
		return
	}
	auth.sendJSONResponse(w, ChallengeResponse{
		Success:   true,
		Challenge: nonce,
		Kind:      LoginProofKind,
		ExpiresIn: int(challengeTTL.Seconds()),
	}, http.StatusOK)
}

// verifyLoginProof checks a signed login event against the pubkey logging in.
// It burns the event's challenge nonce whether or not the rest checks out, so a
// proof can never be replayed.
func verifyLoginProof(ev *nostr.Event, pubkeyHex string, now time.Time) error {
	if ev == nil {
		return fmt.Errorf("a signed login proof is required")
	}
	nonce := tagValue(ev.Tags, "challenge")
	if nonce == "" || !challenges.consume(nonce) {
		return fmt.Errorf("login challenge is missing, expired or already used")
	}
	if ev.Kind != LoginProofKind {
		return fmt.Errorf("login proof must be kind %d", LoginProofKind)
	}
	if !strings.EqualFold(ev.PubKey, pubkeyHex) {
		return fmt.Errorf("login proof was signed by a different key")
	}
	if m := tagValue(ev.Tags, "method"); m != "" && !strings.EqualFold(m, http.MethodPost) {
		return fmt.Errorf("login proof is for the wrong method")
	}
	if u := tagValue(ev.Tags, "u"); u != "" && !strings.HasSuffix(strings.SplitN(u, "?", 2)[0], "/api/auth/login") {
		return fmt.Errorf("login proof is for the wrong URL")
	}
	skew := now.Sub(time.Unix(ev.CreatedAt, 0))
	if skew > proofMaxSkew || skew < -proofMaxSkew {
		return fmt.Errorf("login proof timestamp is too far from server time")
	}
	if !core.VerifyEventSignature(ev) {
		return fmt.Errorf("login proof signature is invalid")
	}
	return nil
}

// tagValue returns the first value of the named tag, or "".
func tagValue(tags [][]string, name string) string {
	for _, t := range tags {
		if len(t) >= 2 && t[0] == name {
			return t[1]
		}
	}
	return ""
}
