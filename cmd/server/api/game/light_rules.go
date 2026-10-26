package game

import (
	"fmt"

	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/types"
)

// Lights at night, out in the wild (degradation §6): a light keeps the dark off
// in a fight, but a light that's shining gives you away. Things that might
// ambush you see you coming (−5 Stealth on the noticing checks). A bullseye
// lantern's long beam helps you see first (its "beam": +2 Perception). A hooded
// lantern with its shutter closed burns on, but hides both its light and you.

// lightStealthPenalty is what a shining light costs your Stealth at night.
const lightStealthPenalty = 5

// lightAtNight is what your light does to the noticing checks right now: the
// Stealth and Perception modifiers, and the log lines that say why. Zero in a
// town, by day, or with no light shining.
func lightAtNight(save *types.SaveFile) (stealth, perception int, notes []string) {
	if save == nil || !types.IsNight(save.TimeOfDay) || travel.GetEnvironmentData(save.Location) == nil {
		return 0, 0, nil
	}
	id, slot, light := condition.HeldLight(save)
	if slot == nil || !condition.Shining(slot) {
		return 0, 0, nil
	}
	name := condition.ItemName(id)
	stealth = -lightStealthPenalty
	notes = append(notes, fmt.Sprintf("  🔦 Your lit %s gives you away in the dark (−%d Stealth).", name, lightStealthPenalty))
	if light.Beam > 0 {
		perception = light.Beam
		notes = append(notes, fmt.Sprintf("  🔦 The %s's beam sweeps ahead (+%d Perception).", name, light.Beam))
	}
	return stealth, perception, notes
}
