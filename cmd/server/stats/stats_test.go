package stats

import (
	"os"
	"path/filepath"
	"testing"

	"pubkey-quest/types"
)

func useStore(t *testing.T) {
	t.Helper()
	oldOn, oldDir := Enabled, Dir
	Enabled, Dir = true, t.TempDir()
	t.Cleanup(func() {
		Enabled, Dir = oldOn, oldDir
		openMu.Lock()
		open = map[string]*Pending{}
		openMu.Unlock()
	})
}

func save(npub, id string) *types.SaveFile {
	return &types.SaveFile{InternalNpub: npub, InternalID: id}
}

func TestApply(t *testing.T) {
	var r Record
	r.Apply([]Op{
		{Kind: opCount, Key: "fish_landed", N: 2},
		{Kind: opCount, Key: "fish_landed", N: 1},
		{Kind: opMember, Key: "fish_species", Member: "river-pike"},
		{Kind: opBest, Key: "fish_oz:river-pike", N: 200, Label: "Mara"},
		{Kind: opBest, Key: "fish_oz:river-pike", N: 150, Label: "Bram"},
	}, 100)
	r.Apply([]Op{
		{Kind: opMember, Key: "fish_species", Member: "river-pike"},
		{Kind: opBest, Key: "fish_oz:river-pike", N: 310, Label: "Bram"},
	}, 200)

	if r.Counters["fish_landed"] != 3 {
		t.Errorf("fish landed %d, want 3", r.Counters["fish_landed"])
	}
	if r.Sets["fish_species"]["river-pike"] != 100 {
		t.Errorf("a set member's first sighting moved: %v", r.Sets["fish_species"])
	}
	if b := r.Bests["fish_oz:river-pike"]; b.Value != 310 || b.Label != "Bram" || b.At != 200 {
		t.Errorf("best %+v, want 310 by Bram at 200", b)
	}
}

// A daily counts once a day for the player, however many characters do it.
func TestCountOncePerPeriod(t *testing.T) {
	useStore(t)
	npub := "npub1player"
	for _, id := range []string{"save_1", "save_2", "save_3"} {
		Open(npub, id, nil)
		CountOnce(save(npub, id), "dailies_completed", "daily-fish-order", "2026-11-21")
		if _, err := Commit(npub, id, 99, nil); err != nil {
			t.Fatal(err)
		}
	}
	Open(npub, "save_1", nil)
	CountOnce(save(npub, "save_1"), "dailies_completed", "daily-ore-order", "2026-11-21")  // another daily
	CountOnce(save(npub, "save_1"), "dailies_completed", "daily-fish-order", "2026-11-22") // the next day
	Commit(npub, "save_1", 99, nil)

	r, _ := Load(npub)
	if r.Counters["dailies_completed"] != 3 {
		t.Fatalf("dailies %d, want 3 (fish order on two days, ore order once)", r.Counters["dailies_completed"])
	}
}

// Only what a kept save covers counts: a reload drops the rest, and play
// after a save's mark waits for the next save.
func TestCommitGate(t *testing.T) {
	useStore(t)
	npub, id := "npub1player", "save_1"
	s := save(npub, id)
	p := Open(npub, id, nil)

	Count(s, "monsters_slain", 1)
	Open(npub, id, nil) // reloaded without saving
	if _, err := Commit(npub, id, 99, nil); err != nil {
		t.Fatal(err)
	}
	if r, _ := Load(npub); r.Counters["monsters_slain"] != 0 {
		t.Fatalf("a reload kept %d kills", r.Counters["monsters_slain"])
	}

	p = pendingFor(s)
	Count(s, "monsters_slain", 2)
	mark := p.Mark() // a relay save prepared here
	Count(s, "monsters_slain", 5)
	Commit(npub, id, mark, nil)
	if r, _ := Load(npub); r.Counters["monsters_slain"] != 2 {
		t.Fatalf("after the first save %d kills, want 2", r.Counters["monsters_slain"])
	}
	Commit(npub, id, p.Mark(), nil)
	if r, _ := Load(npub); r.Counters["monsters_slain"] != 7 {
		t.Fatalf("after the second save %d kills, want 7", r.Counters["monsters_slain"])
	}
}

// A crash journal's ops come back with the session.
func TestOpenRestores(t *testing.T) {
	useStore(t)
	npub, id := "npub1player", "save_1"
	Open(npub, id, nil)
	AddMember(save(npub, id), "fish_species", "carp")
	kept := pendingFor(save(npub, id)).Snapshot()

	p := Open(npub, id, kept)
	Commit(npub, id, p.Mark(), nil)
	if r, _ := Load(npub); r.Sets["fish_species"]["carp"] == 0 {
		t.Fatal("a journaled catch was lost")
	}
}

// A commit that can't be written keeps its stats for the next save.
func TestFailedCommitKeepsPending(t *testing.T) {
	useStore(t)
	npub, id := "npub1player", "save_1"
	p := Open(npub, id, nil)
	Count(save(npub, id), "fish_landed", 4)
	good := Dir
	Dir = filepath.Join(t.TempDir(), "missing", "\x00bad")
	if _, err := Commit(npub, id, p.Mark(), nil); err == nil {
		t.Fatal("expected the write to fail")
	}
	Dir = good

	// The record reads but can't be written: the ops go back.
	blocker := filepath.Join(Dir, npub+".json.tmp")
	if err := os.MkdirAll(blocker, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Commit(npub, id, p.Mark(), nil); err == nil {
		t.Fatal("expected the write to fail")
	}
	os.Remove(blocker)

	Commit(npub, id, p.Mark(), nil)
	if r, _ := Load(npub); r.Counters["fish_landed"] != 4 {
		t.Fatalf("fish landed %d after the retry, want 4", r.Counters["fish_landed"])
	}
}

func TestOff(t *testing.T) {
	useStore(t)
	Enabled = false
	if Open("npub1player", "save_1", nil) != nil {
		t.Fatal("the store opened while off")
	}
	Count(save("npub1player", "save_1"), "fish_landed", 1) // must not panic
	if r, err := Commit("npub1player", "save_1", 9, nil); err != nil || r.Counters != nil {
		t.Fatalf("a commit while off: %+v %v", r, err)
	}
	var p *Pending
	if p.Mark() != 0 || p.Snapshot() != nil {
		t.Fatal("a nil Pending isn't safe")
	}
}

// The highscores: every player's score on a board, highest first; a tie goes
// to whoever got there first. A record written later shows straight away.
func TestTop(t *testing.T) {
	useStore(t)
	ResetBoards()
	t.Cleanup(ResetBoards)
	for npub, s := range map[string]Score{
		"npub_a": {Entry: Entry{Value: 500}, At: 20},
		"npub_b": {Entry: Entry{Value: 900}, At: 30},
		"npub_c": {Entry: Entry{Value: 500}, At: 10},
	} {
		if err := write(npub, Record{Scores: map[string]Score{"xp": s}}); err != nil {
			t.Fatal(err)
		}
	}
	got := Top("xp", 0)
	if len(got) != 3 || got[0].Npub != "npub_b" || got[1].Npub != "npub_c" || got[2].Npub != "npub_a" {
		t.Fatalf("board %+v, want b, c (first to 500), a", got)
	}
	write("npub_a", Record{Scores: map[string]Score{"xp": {Entry: Entry{Value: 1000}, At: 40}}})
	if top := Top("xp", 1); len(top) != 1 || top[0].Npub != "npub_a" {
		t.Fatalf("after a's new score: %+v", top)
	}
}
