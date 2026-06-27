package events_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/types"
)

func TestRecorderFansOutToAllConsumers(t *testing.T) {
	var r events.Recorder
	var a, b []events.Event
	r.Subscribe(func(_ *types.SaveFile, ev events.Event) { a = append(a, ev) })
	r.Subscribe(func(_ *types.SaveFile, ev events.Event) { b = append(b, ev) })

	save := &types.SaveFile{}
	r.Record(save, events.MonsterKilled, "wolf", 2)

	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("both consumers should receive the event, got a=%d b=%d", len(a), len(b))
	}
	if a[0].Kind != events.MonsterKilled || a[0].Target != "wolf" || a[0].Count != 2 {
		t.Errorf("unexpected event delivered: %+v", a[0])
	}
}

func TestRecordNormalisesCount(t *testing.T) {
	var r events.Recorder
	var got events.Event
	r.Subscribe(func(_ *types.SaveFile, ev events.Event) { got = ev })

	r.Record(&types.SaveFile{}, events.NPCTalked, "kingdom-inn-keeper", 0)
	if got.Count != 1 {
		t.Errorf("count <= 0 should normalise to 1, got %d", got.Count)
	}
}

func TestConsumerSeesSaveAndCanMutate(t *testing.T) {
	var r events.Recorder
	// A consumer that mutates the save proves consumers can drive save state
	// (the pattern the quest objective tracker will use).
	r.Subscribe(func(s *types.SaveFile, ev events.Event) {
		if ev.Kind == events.ItemAcquired {
			s.Experience += ev.Count
		}
	})

	save := &types.SaveFile{Experience: 10}
	r.Record(save, events.ItemAcquired, "healing-potion", 3)
	if save.Experience != 13 {
		t.Errorf("consumer should have mutated the save, got Experience=%d", save.Experience)
	}
}

func TestDefaultRecorder(t *testing.T) {
	events.Reset()
	t.Cleanup(events.Reset)

	var seen int
	events.Subscribe(func(_ *types.SaveFile, _ events.Event) { seen++ })
	events.Record(&types.SaveFile{}, events.LocationDiscovered, "old-hunters-shack", 1)

	if seen != 1 {
		t.Errorf("default recorder should deliver to subscribed consumer, seen=%d", seen)
	}
}
