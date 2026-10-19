// Package teleport is Homeward Stones and the waystone network
// (docs/draft/teleportation-design.md, game-data/systems/teleportation.json).
//
//   - A Homeward Stone (an item with "teleport": {"city": …}) takes you to its
//     city's waystone. All stones share one cooldown (an ActiveEffect).
//   - Standing at a waystone, you can hop to any other waystone you've woken
//     (that city's waystone quest completed). Hops have their own cooldown.
//   - A city's stone is for sale from its vault keeper to anyone who started
//     there, or who has done that city's stone quest — but not while you
//     already hold one (in your packs or your vault).
//
// Nothing new is saved: unlocks derive from QuestsCompleted, cooldowns are
// effects, stones are items. Teleporting never advances the clock.
package teleport

import (
	"encoding/json"
	"fmt"
	"sync"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/cmd/server/game/vault"
	"pubkey-quest/types"
)

// City is one waystone city.
type City struct {
	Name          string `json:"name"`
	District      string `json:"district"`       // where its waystone stands
	Stone         string `json:"stone"`          // its Homeward Stone item id
	Keeper        string `json:"keeper"`         // the vault keeper who sells the stone
	StoneQuest    string `json:"stone_quest"`    // lets others buy the stone
	WaystoneQuest string `json:"waystone_quest"` // wakes its waystone as a hop destination
}

// Config mirrors teleportation.json.
type Config struct {
	StoneCooldownEffect    string          `json:"stone_cooldown_effect"`
	WaystoneCooldownEffect string          `json:"waystone_cooldown_effect"`
	StonePrice             int             `json:"stone_price"`
	Chain                  []string        `json:"chain"` // the waystone quest order
	Cities                 map[string]City `json:"cities"`
}

var (
	mu  sync.Mutex
	cfg *Config
)

// Load returns the config, read once the database is open (not cached before).
func Load() *Config {
	mu.Lock()
	defer mu.Unlock()
	if cfg != nil {
		return cfg
	}
	c := &Config{Cities: map[string]City{}}
	database := db.GetDB()
	if database == nil {
		return c
	}
	var blob string
	if database.QueryRow("SELECT data FROM teleportation WHERE id = 'teleportation'").Scan(&blob) != nil {
		return c
	}
	if json.Unmarshal([]byte(blob), c) != nil {
		return c
	}
	cfg = c
	return c
}

// SetConfig replaces the config (tests).
func SetConfig(c *Config) {
	mu.Lock()
	cfg = c
	mu.Unlock()
}

// ─── Who owns what ──────────────────────────────────────────────────────────

// HomeCity is where the character started (LocationsDiscovered[0], which death
// also returns you to).
func HomeCity(save *types.SaveFile) string {
	if len(save.LocationsDiscovered) == 0 {
		return ""
	}
	return save.LocationsDiscovered[0]
}

// StoneCity is the city a Homeward Stone item takes you to ("" if it isn't one).
func StoneCity(itemID string) string {
	for id, c := range Load().Cities {
		if c.Stone == itemID {
			return id
		}
	}
	return ""
}

// Holds reports whether the character has the item in their packs or vault.
func Holds(save *types.SaveFile, itemID string) bool {
	return gameutil.CountItem(save, itemID) > 0 || vault.Quantity(save, itemID) > 0 || equipped(save, itemID)
}

func equipped(save *types.SaveFile, itemID string) bool {
	gear, _ := save.Inventory["gear_slots"].(map[string]interface{})
	for _, raw := range gear {
		if s, ok := raw.(map[string]interface{}); ok {
			if id, _ := s["item"].(string); id == itemID {
				return true
			}
		}
	}
	return false
}

// StoneForSale reports whether a keeper will sell the character this city's
// stone, and why not. Natives can always (re)buy theirs; others need the
// city's stone quest. Nobody can buy a stone they already hold.
func StoneForSale(save *types.SaveFile, city string) (bool, string) {
	c, ok := Load().Cities[city]
	if !ok {
		return false, "that isn't a Homeward Stone"
	}
	if HomeCity(save) != city && !quest.IsCompleted(save, c.StoneQuest) {
		return false, fmt.Sprintf("The keeper won't attune a stone of %s to a stranger. Earn their trust first.", c.Name)
	}
	if Holds(save, c.Stone) {
		return false, "You already have that stone (or it's in your vault)."
	}
	return true, ""
}

// ─── Cooldowns ──────────────────────────────────────────────────────────────

// CooldownLeft is the minutes left on a cooldown effect (0 when ready).
func CooldownLeft(save *types.SaveFile, effectID string) int {
	for _, ae := range save.ActiveEffects {
		if ae.EffectID == effectID && ae.DurationRemaining > 0 {
			return int(ae.DurationRemaining + 0.999)
		}
	}
	return 0
}

// FormatWait renders a cooldown for players ("6 days 4 hours", "35 minutes").
func FormatWait(minutes int) string {
	d, h, m := minutes/1440, (minutes%1440)/60, minutes%60
	plural := func(n int, w string) string {
		if n == 1 {
			return fmt.Sprintf("1 %s", w)
		}
		return fmt.Sprintf("%d %ss", n, w)
	}
	switch {
	case d > 0 && h > 0:
		return plural(d, "day") + " " + plural(h, "hour")
	case d > 0:
		return plural(d, "day")
	case h > 0:
		return plural(h, "hour")
	}
	return plural(max(m, 1), "minute")
}

// ─── Waystones ──────────────────────────────────────────────────────────────

// WaystoneHere is the city whose waystone the character stands at (in its
// district, outside any building), or "".
func WaystoneHere(save *types.SaveFile) string {
	c, ok := Load().Cities[save.Location]
	if !ok || save.Building != "" || save.District != c.District {
		return ""
	}
	return save.Location
}

// Destination is a waystone you might hop to.
type Destination struct {
	City     string `json:"city"`
	Name     string `json:"name"`
	Unlocked bool   `json:"unlocked"`
	Quest    string `json:"quest"` // the quest that wakes it
}

// Destinations lists every waystone in chain order, marked woken or not.
func Destinations(save *types.SaveFile) []Destination {
	c := Load()
	var out []Destination
	for _, id := range c.Chain {
		city := c.Cities[id]
		out = append(out, Destination{City: id, Name: city.Name, Unlocked: quest.IsCompleted(save, city.WaystoneQuest), Quest: city.WaystoneQuest})
	}
	return out
}

// ─── Teleporting ────────────────────────────────────────────────────────────

// Result is where a teleport took the character.
type Result struct {
	City       string   `json:"city"`
	Name       string   `json:"name"`
	Discovered bool     `json:"discovered"`      // first time in this city
	Music      []string `json:"music,omitempty"` // tracks unlocked by arriving
	Message    string   `json:"message"`
}

// UseStone teleports the character with a Homeward Stone they carry. The
// caller has already checked they aren't fighting, in a POI, or in a party.
func UseStone(save *types.SaveFile, itemID string) (*Result, error) {
	c := Load()
	city := StoneCity(itemID)
	if city == "" {
		return nil, fmt.Errorf("that isn't a Homeward Stone")
	}
	if gameutil.CountItem(save, itemID) == 0 {
		return nil, fmt.Errorf("you need the stone in hand (your packs, not the vault)")
	}
	if left := CooldownLeft(save, c.StoneCooldownEffect); left > 0 {
		return nil, fmt.Errorf("your Homeward Stones are still recharging: %s to go", FormatWait(left))
	}
	if save.Location == city && save.District == c.Cities[city].District && save.Building == "" {
		return nil, fmt.Errorf("you're already standing at %s's waystone", c.Cities[city].Name)
	}
	res := arrive(save, city)
	if err := effects.ApplyEffect(save, c.StoneCooldownEffect); err != nil {
		return nil, err
	}
	res.Message = fmt.Sprintf("You close your hand around the stone. The world folds, and you're standing at the waystone in %s.", res.Name)
	return res, nil
}

// Hop carries the character from the waystone they stand at to another woken
// waystone.
func Hop(save *types.SaveFile, dest string) (*Result, error) {
	c := Load()
	here := WaystoneHere(save)
	if here == "" {
		return nil, fmt.Errorf("you need to be standing at a waystone")
	}
	d, ok := c.Cities[dest]
	if !ok {
		return nil, fmt.Errorf("there's no waystone there")
	}
	if dest == here {
		return nil, fmt.Errorf("you're already here")
	}
	if !quest.IsCompleted(save, d.WaystoneQuest) {
		return nil, fmt.Errorf("the waystone in %s is still dormant", d.Name)
	}
	if left := CooldownLeft(save, c.WaystoneCooldownEffect); left > 0 {
		return nil, fmt.Errorf("the waystones won't carry you again for %s", FormatWait(left))
	}
	res := arrive(save, dest)
	if err := effects.ApplyEffect(save, c.WaystoneCooldownEffect); err != nil {
		return nil, err
	}
	res.Message = fmt.Sprintf("The runes flare. You step through light and out at the waystone in %s.", res.Name)
	return res, nil
}

// arrive moves the character to a city's waystone: any trip on the road is
// abandoned, and the city is discovered as if they'd walked in. No time passes.
func arrive(save *types.SaveFile, city string) *Result {
	c := Load().Cities[city]
	save.Location, save.District, save.Building, save.Room = city, c.District, "", ""
	save.TravelProgress, save.TravelStopped = 0, false
	discovered, music := travel.RecordArrival(save, city)
	return &Result{City: city, Name: c.Name, Discovered: discovered, Music: music}
}
