package session

import (
	"fmt"
	"regexp"

	"github.com/0ceanslim/grain/client/core/tools"
)

// A character slot is (npub, save id), and both become file paths: saves,
// journals, signed events and the ledger all live under data/<…>/<npub>/
// <save id>. Requests name them, so they're checked here, where every session
// is loaded or looked up, and nothing else reaches the disk. One spelling per
// player (the npub, never hex) also means one session and one ledger slot per
// character, not a second one under another name.

var saveIDPattern = regexp.MustCompile(`^save_[0-9]{1,20}$`)

// ValidSaveID reports whether a save id is one the game makes ("save_<unix>").
func ValidSaveID(saveID string) bool { return saveIDPattern.MatchString(saveID) }

// ValidNpub reports whether npub is a player's key in its one spelling: a
// bech32 npub that encodes back to itself.
func ValidNpub(npub string) bool {
	hex, err := tools.DecodeNpub(npub)
	if err != nil {
		return false
	}
	canon, err := tools.EncodePubkey(hex)
	return err == nil && canon == npub
}

// ValidSlot checks a slot as a request names it.
func ValidSlot(npub, saveID string) error {
	if !ValidSaveID(saveID) {
		return fmt.Errorf("invalid save id")
	}
	if !ValidNpub(npub) {
		return fmt.Errorf("invalid npub")
	}
	return nil
}
