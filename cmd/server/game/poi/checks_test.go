package poi

import (
	"testing"

	"pubkey-quest/types"
)

func TestCheckTriesCloseAfterAttemptsThenReopen(t *testing.T) {
	node := types.POIStep{Type: types.POIStepCheck, Attempts: 2, CooldownMinutes: 60}
	tries := CheckTries{}

	if left := tries.Record("w/n", node, false, 100); left != 1 {
		t.Fatalf("after one failure: %d left, want 1", left)
	}
	if closed, _ := tries.Closed("w/n", 100); closed {
		t.Fatal("one failure of two must not close the check")
	}
	if left := tries.Record("w/n", node, false, 101); left != 0 {
		t.Fatalf("after the last failure: %d left, want 0", left)
	}
	if closed, wait := tries.Closed("w/n", 130); !closed || wait != 31 {
		t.Fatalf("spent check should be closed with 31 min left, got %v %d", closed, wait)
	}
	if closed, _ := tries.Closed("w/n", 161); closed {
		t.Fatal("the check should reopen once the cooldown is up")
	}
	if left := tries.Record("w/n", node, false, 161); left != 1 {
		t.Fatalf("a reopened check starts a fresh run: %d left, want 1", left)
	}
	tries.Record("w/n", node, true, 162)
	if _, ok := tries["w/n"]; ok {
		t.Error("a success should clear the run")
	}
}

func TestCheckDefaults(t *testing.T) {
	n := types.POIStep{Type: types.POIStepCheck}
	if CheckAttempts(n) != DefaultCheckAttempts || CheckCooldown(n) != DefaultCheckCooldown {
		t.Error("unset attempts/cooldown should take the defaults")
	}
}
