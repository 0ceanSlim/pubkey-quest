// Package social holds the in-memory parts of playing together: chat
// mailboxes and parties (docs/draft/social-plan.md §6, §9). Nothing here is
// stored or published; it lives as long as the server process.
package social

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

// ChatLine is one message delivered to a player.
type ChatLine struct {
	ID       int64  `json:"id"`
	Channel  string `json:"channel"` // say | area | party | system
	FromNpub string `json:"from_npub"`
	FromName string `json:"from_name"`
	Text     string `json:"text"`
	At       int64  `json:"at"` // unix seconds
}

const (
	// MaxChatLength caps a line, in characters.
	MaxChatLength = 200
	mailboxSize   = 50
	rateWindow    = 10 * time.Second
	rateLines     = 5
)

var (
	chatMu    sync.Mutex
	mailboxes = map[string][]ChatLine{} // npub → undelivered lines (a ring of the last 50)
	sentAt    = map[string][]time.Time{} // npub → recent send times, for the rate limit
	nextID    atomic.Int64
)

// CleanText trims a chat line, strips control characters and caps its length.
// "" means nothing worth sending.
func CleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > MaxChatLength {
		s = string(r[:MaxChatLength])
	}
	return s
}

// AllowSend applies the per-sender rate limit (5 lines per 10 s) and records
// the send when allowed.
func AllowSend(npub string, now time.Time) bool {
	chatMu.Lock()
	defer chatMu.Unlock()
	recent := sentAt[npub][:0]
	for _, t := range sentAt[npub] {
		if now.Sub(t) < rateWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) >= rateLines {
		sentAt[npub] = recent
		return false
	}
	sentAt[npub] = append(recent, now)
	return true
}

// Deliver drops a line into each recipient's mailbox.
func Deliver(line ChatLine, recipients []string) {
	line.ID = nextID.Add(1)
	if line.At == 0 {
		line.At = time.Now().Unix()
	}
	chatMu.Lock()
	defer chatMu.Unlock()
	for _, r := range recipients {
		box := append(mailboxes[r], line)
		if len(box) > mailboxSize {
			box = box[len(box)-mailboxSize:]
		}
		mailboxes[r] = box
	}
}

// Drain returns and clears a player's undelivered lines.
func Drain(npub string) []ChatLine {
	chatMu.Lock()
	defer chatMu.Unlock()
	out := mailboxes[npub]
	delete(mailboxes, npub)
	if out == nil {
		return []ChatLine{}
	}
	return out
}

// Forget drops a player's mailbox and rate-limit history (logout).
func Forget(npub string) {
	chatMu.Lock()
	delete(mailboxes, npub)
	delete(sentAt, npub)
	chatMu.Unlock()
}
