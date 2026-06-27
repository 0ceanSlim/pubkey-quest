// Package discovery awards experience for exploring the world. It plugs into
// the event recorder as a second consumer (alongside the quest tracker), so a
// newly discovered location or environment grants a flat exploration reward on
// top of its music unlock.
package discovery

import (
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/types"
)

// BaseXP is the flat experience awarded for discovering a new location or
// environment. Tunable — exploration should feel rewarded, not be a grind path.
const BaseXP = 25

// XPConsumer returns an events.Consumer that grants BaseXP whenever a new
// location or environment is discovered. Register once at startup with the
// loaded advancement table (so the reward can roll a level-up).
func XPConsumer(advancement []types.AdvancementEntry) events.Consumer {
	return func(save *types.SaveFile, ev events.Event) {
		if ev.Kind == events.LocationDiscovered {
			character.GrantXP(save, BaseXP, advancement)
		}
	}
}
