package discovery_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/discovery"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/types"
)

func TestDiscoveryGrantsXP(t *testing.T) {
	var r events.Recorder
	r.Subscribe(discovery.XPConsumer(nil))

	save := &types.SaveFile{Experience: 100}
	r.Record(save, events.LocationDiscovered, "darkwood-forest", 1)

	if save.Experience != 100+discovery.BaseXP {
		t.Errorf("discovery should grant %d XP: Experience = %d, want %d",
			discovery.BaseXP, save.Experience, 100+discovery.BaseXP)
	}
}

func TestNonDiscoveryEventsDoNotGrantXP(t *testing.T) {
	var r events.Recorder
	r.Subscribe(discovery.XPConsumer(nil))

	save := &types.SaveFile{Experience: 100}
	r.Record(save, events.MonsterKilled, "wolf", 1)
	r.Record(save, events.NPCTalked, "innkeeper", 1)

	if save.Experience != 100 {
		t.Errorf("only discovery should grant XP, got Experience = %d", save.Experience)
	}
}
