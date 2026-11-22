package dm

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/core/tools"
	nostr "github.com/0ceanslim/grain/server/types"

	"pubkey-quest/cmd/server/game/badges"
	"pubkey-quest/cmd/server/game/boards"
	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/cmd/server/utils"
	"pubkey-quest/types"
)

// sent is what a test's relays received.
type sent struct {
	events []*nostr.Event
	fail   bool
}

// asDM makes this test's server the official one, the DM signing with a
// fresh key into relays that record what they're sent.
func asDM(t *testing.T) *sent {
	t.Helper()
	kp, err := tools.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	s, _ := core.NewEventSigner(kp.PrivateKey)
	oldSigner, oldPub, oldPublish := signer, pubHex, publishFn
	oldOn, oldDir, oldDefs := stats.Enabled, stats.Dir, DefinitionsFile
	signer, pubHex = s, kp.PublicKey
	stats.Enabled, stats.Dir = true, t.TempDir()
	DefinitionsFile = filepath.Join(t.TempDir(), "definitions.json")
	got := &sent{}
	publishFn = func(_ context.Context, ev *nostr.Event, relays []string) []string {
		if got.fail {
			return nil
		}
		got.events = append(got.events, ev)
		return []string{"wss://test"}
	}
	t.Cleanup(func() {
		signer, pubHex, publishFn = oldSigner, oldPub, oldPublish
		stats.Enabled, stats.Dir, DefinitionsFile = oldOn, oldDir, oldDefs
	})
	return got
}

func (s *sent) kinds() map[int]int {
	out := map[int]int{}
	for _, e := range s.events {
		out[e.Kind]++
	}
	return out
}

func tag(ev *nostr.Event, name string) []string {
	for _, t := range ev.Tags {
		if len(t) > 0 && t[0] == name {
			return t
		}
	}
	return nil
}

func TestEvents(t *testing.T) {
	def := Definition(types.BadgeDef{ID: "every-fish", Name: "Angler of the Realm", Description: "Caught one of every fish."},
		types.BadgeMedia{Image: "https://b/full.png", Thumb256: "https://b/256.png", Thumb64: "https://b/64.png"})
	if def.Kind != 30009 || tag(def, "d")[1] != "every-fish" || tag(def, "image")[2] != "1024x1024" {
		t.Errorf("definition %+v", def.Tags)
	}
	award := Award("every-fish", "dmhex", "playerhex")
	if award.Kind != 8 || tag(award, "a")[1] != "30009:dmhex:every-fish" || tag(award, "p")[1] != "playerhex" {
		t.Errorf("award %+v", award.Tags)
	}
	score := Score(boards.XP, stats.Score{Entry: stats.Entry{Value: 23400, Who: "Mara", Detail: "level 7 Druid"}}, "playerhex")
	if score.Kind != 30762 || tag(score, "d")[1] != "pubkey-quest:playerhex:xp" || tag(score, "score")[1] != "23400" ||
		tag(score, "level")[1] != "xp" || tag(score, "game")[1] != "pubkey-quest" {
		t.Errorf("score %+v", score.Tags)
	}
	if score.Content != "Mara, level 7 Druid: 23,400 XP." {
		t.Errorf("score line %q", score.Content)
	}
}

func TestScoreLines(t *testing.T) {
	for _, c := range []struct {
		board string
		s     stats.Score
		want  string
	}{
		{boards.Biggest + "river-pike", stats.Score{Entry: stats.Entry{Value: 310, Who: "Mara", Detail: "river-pike"}}, "Mara landed a 19 lb 6 oz river pike."},
		{boards.Toughest, stats.Score{Entry: stats.Entry{Value: 1800, Who: "Bram", Detail: "owlbear"}}, "Bram slew an owlbear, worth 1,800 XP."},
		{boards.Slain, stats.Score{Entry: stats.Entry{Value: 1234567}}, "1,234,567 monsters slain."},
		{boards.Biggest + "minnow", stats.Score{Entry: stats.Entry{Value: 3, Who: "Mara", Detail: "minnow"}}, "Mara landed a 3 oz minnow."},
	} {
		if got := ScoreLine(c.board, c.s); got != c.want {
			t.Errorf("%s: %q, want %q", c.board, got, c.want)
		}
	}
}

// A sweep signs each award and changed score once, mirrors the record, and
// leaves nothing for the next sweep; what no relay took is tried again.
func TestSweep(t *testing.T) {
	got := asDM(t)
	kp, _ := tools.GenerateKeyPair()
	npub := kp.Npub
	err := stats.Update(npub, func(r *stats.Record) bool {
		r.Badges = map[string]int64{"level-5": 1, "every-fish": 2}
		r.Scores = map[string]stats.Score{boards.XP: {Entry: stats.Entry{Value: 900, Who: "Mara"}, At: 5}}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}

	got.fail = true // the relays are down
	SweepPlayer(npub)
	if r, _ := stats.Load(npub); len(r.Awarded) != 0 || r.Scores[boards.XP].Event != "" || r.Mirrored != 0 {
		t.Fatalf("nothing was taken, but the record says %+v", r)
	}

	got.fail = false
	SweepPlayer(npub)
	if k := got.kinds(); k[8] != 2 || k[30762] != 1 || k[30078] != 1 {
		t.Fatalf("sent kinds %v, want 2 awards, 1 score, 1 record", k)
	}
	r, _ := stats.Load(npub)
	if len(r.Awarded) != 2 || r.Scores[boards.XP].Event == "" || r.Mirrored != r.Updated {
		t.Fatalf("after the sweep: %+v", r)
	}
	for _, ev := range got.events {
		if ev.PubKey != pubHex || !core.VerifyEventSignature(ev) {
			t.Fatalf("kind %d isn't signed by the DM", ev.Kind)
		}
		if ev.Kind == 30078 {
			var mirrored stats.Record
			if json.Unmarshal([]byte(ev.Content), &mirrored) != nil || len(mirrored.Badges) != 2 {
				t.Fatalf("the mirrored record: %s", ev.Content)
			}
		}
	}

	n := len(got.events)
	SweepPlayer(npub)
	if len(got.events) != n {
		t.Fatalf("a second sweep sent %d more events", len(got.events)-n)
	}
}

// Badge definitions are signed once, and again only when one changes.
func TestPublishDefinitions(t *testing.T) {
	got := asDM(t)
	defs := &types.BadgesConfig{Badges: []types.BadgeDef{
		{ID: "level-5", Name: "Adventurer's Mark", Description: "Reached level 5."},
		{ID: "level-10", Name: "Sworn Blade", Description: "Reached level 10."},
	}}
	badges.SetConfig(defs, nil)
	badges.SetMedia(&types.BadgeMediaConfig{Badges: map[string]types.BadgeMedia{"level-5": {Image: "https://b/5.png"}}})

	PublishDefinitions()
	PublishDefinitions()
	if got.kinds()[30009] != 2 {
		t.Fatalf("published %d definitions, want 2 (once each)", got.kinds()[30009])
	}
	defs.Badges[0].Description = "Reached level five."
	PublishDefinitions()
	if got.kinds()[30009] != 3 {
		t.Fatalf("a changed badge wasn't published again: %d", got.kinds()[30009])
	}
}

// Test (no saves.validation) never signs.
func TestTestServerNeverSigns(t *testing.T) {
	kp, _ := tools.GenerateKeyPair()
	oldCfg, oldOn, oldSigner := utils.AppConfig, stats.Enabled, signer
	t.Cleanup(func() { utils.AppConfig, stats.Enabled, signer = oldCfg, oldOn, oldSigner })
	signer = nil
	utils.AppConfig.DM = utils.DMConfig{Npub: kp.Npub, Nsec: kp.Nsec}
	utils.AppConfig.Saves.Validation = false
	stats.Enabled = true
	if Init() || Signs() {
		t.Fatal("the DM signs on a server without saves.validation")
	}
}
