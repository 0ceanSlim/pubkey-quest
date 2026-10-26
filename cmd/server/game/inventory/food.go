package inventory

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/types"
)

// Eating (docs/draft/food-system.md): food keeps you fed for a number of hours,
// "food": {"hours": 4}. Stale food is half as filling. Risky food (raw meat,
// spoiled food), "food": {"hours": 1, "risky_dc": 10}, asks a Constitution save
// or gives food poisoning, and while you have food poisoning you can't eat.

// FoodPoisoning is the effect bad food gives.
const FoodPoisoning = "food-poisoning"

// Food is an item's "food" block.
type Food struct {
	Hours   float64 `json:"hours"`
	RiskyDC int     `json:"risky_dc,omitempty"`
}

// FoodOf returns an item's food block, or nil if it isn't food.
func FoodOf(itemID string) *Food {
	item, err := db.GetItemByID(itemID)
	if err != nil || item == nil {
		return nil
	}
	var props struct {
		Food *Food `json:"food"`
	}
	if json.Unmarshal([]byte(item.Properties), &props) != nil {
		return nil
	}
	return props.Food
}

// WhyCantEat is the reason a food can't be eaten right now ("" when it can):
// you're full, or food poisoning won't let you keep it down.
func WhyCantEat(state *types.SaveFile, itemID string) string {
	if FoodOf(itemID) == nil {
		return ""
	}
	if effects.HasActiveEffect(state, FoodPoisoning) {
		return "Your stomach won't take anything until the food poisoning passes."
	}
	if status.IsFull(state) {
		return "You couldn't manage another bite."
	}
	return ""
}

// eat feeds you from a food: its hours (half if stale), then the save for risky
// food. slot is the inventory slot it came from (for freshness); nil means fresh.
func eat(state *types.SaveFile, itemID string, food *Food, slot map[string]interface{}) []string {
	minutes := int(food.Hours * 60)
	stale := false
	if slot != nil {
		if f, _ := condition.FoodFreshness(slot, condition.Now(state)); f == condition.Stale {
			minutes /= 2
			stale = true
		}
	}
	gained, wasted := status.Feed(state, minutes)
	msg := "Keeps you fed " + hoursLabel(gained)
	if stale {
		msg += " (it's stale: half as filling)"
	}
	if wasted > 0 {
		msg += fmt.Sprintf("; you couldn't finish it (%s wasted)", hoursLabel(wasted))
	}
	msgs := []string{msg}

	if food.RiskyDC > 0 {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		check := skillcheck.Resolve(statScore(state, "constitution"), food.RiskyDC, rng)
		if check.Success {
			msgs = append(msgs, fmt.Sprintf("Constitution save %d vs DC %d: it stays down.", check.Total, check.DC))
		} else if err := effects.ApplyEffect(state, FoodPoisoning); err == nil {
			msgs = append(msgs, fmt.Sprintf("Constitution save %d vs DC %d: food poisoning.", check.Total, check.DC))
		}
	}
	return msgs
}

// hoursLabel writes minutes as hours and minutes ("4h", "1h 30m", "45m").
func hoursLabel(minutes int) string {
	h, m := minutes/60, minutes%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// HoursLabel is hoursLabel for callers outside the package ("8h", "1h 30m").
func HoursLabel(minutes int) string { return hoursLabel(minutes) }

// statScore reads an effective ability score (buffs and penalties included).
func statScore(state *types.SaveFile, stat string) int {
	stats := effects.EffectiveStats(state)
	for k, v := range stats {
		if strings.EqualFold(k, stat) {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			}
		}
	}
	return 10
}
