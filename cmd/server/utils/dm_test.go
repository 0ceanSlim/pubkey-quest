package utils

import (
	"testing"

	"github.com/0ceanslim/grain/client/core/tools"
)

func TestDMKey(t *testing.T) {
	if _, _, ok, err := (DMConfig{}).DMKey(); ok || err != nil {
		t.Fatalf("no DM: ok=%v err=%v", ok, err)
	}
	kp, _ := tools.GenerateKeyPair()
	other, _ := tools.GenerateKeyPair()
	priv, pub, ok, err := DMConfig{Npub: kp.Npub, Nsec: kp.Nsec}.DMKey()
	if err != nil || !ok || priv != kp.PrivateKey || pub != kp.PublicKey {
		t.Fatalf("a matching key: ok=%v err=%v", ok, err)
	}
	if _, _, _, err := (DMConfig{Npub: other.Npub, Nsec: kp.Nsec}).DMKey(); err == nil {
		t.Fatal("an nsec that isn't the npub's passed")
	}
	if _, _, _, err := (DMConfig{Npub: kp.Npub}).DMKey(); err == nil {
		t.Fatal("an npub with no nsec passed")
	}
	if _, _, _, err := (DMConfig{Npub: kp.Npub, Nsec: kp.Npub}).DMKey(); err == nil {
		t.Fatal("an npub given as the nsec passed")
	}
}
