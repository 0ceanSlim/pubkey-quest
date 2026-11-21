package utils

import (
	"encoding/hex"
	"fmt"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcutil/bech32"
)

// DMKey returns the Dungeon Master's private and public keys (hex), checking
// that the nsec is his npub's. ok is false when no DM is set. The nsec is
// decoded here rather than with tools.DecodeNsec, which logs what it decodes.
func (c DMConfig) DMKey() (priv, pub string, ok bool, err error) {
	if c.Nsec == "" && c.Npub == "" {
		return "", "", false, nil
	}
	if c.Nsec == "" || c.Npub == "" {
		return "", "", false, fmt.Errorf("dm needs both npub and nsec")
	}
	hrp, data, err := bech32.Decode(c.Nsec)
	if err != nil || hrp != "nsec" {
		return "", "", false, fmt.Errorf("dm.nsec isn't an nsec")
	}
	raw, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil || len(raw) != 32 {
		return "", "", false, fmt.Errorf("dm.nsec isn't an nsec")
	}
	_, pk := btcec.PrivKeyFromBytes(raw)
	pub = hex.EncodeToString(schnorr.SerializePubKey(pk))
	want, err := tools.DecodeNpub(c.Npub)
	if err != nil || want != pub {
		return "", "", false, fmt.Errorf("dm.nsec isn't the key of dm.npub")
	}
	return hex.EncodeToString(raw), pub, true, nil
}
