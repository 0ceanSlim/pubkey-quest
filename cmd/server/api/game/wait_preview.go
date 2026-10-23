package game

import (
	"encoding/json"
	"net/http"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/cmd/server/game/spells"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// Wait lengths the Wait window offers (its slider: 15 minutes to 6 hours).
const (
	waitStepMinutes = 15
	waitMaxMinutes  = 360
)

// WaitStep is what waiting a given number of minutes would do, worked out by
// running the real wait on a copy of the save, so the Wait window can't drift
// from the rules.
type WaitStep struct {
	Minutes     int      `json:"minutes"`
	TimeOfDay   int      `json:"time_of_day"`
	Day         int      `json:"day"`
	HP          int      `json:"hp"`
	Mana        int      `json:"mana"`
	Hunger      int      `json:"hunger"`
	HungerName  string   `json:"hunger_name"`
	HPLost      int      `json:"hp_lost,omitempty"`      // HP lost along the way (starving, poison); HP is net
	EffectsEnd  []string `json:"effects_end,omitempty"`  // visible effects that wear off
	EffectsGain []string `json:"effects_gain,omitempty"` // visible effects that set in (Hungry…)
	SpellsReady []string `json:"spells_ready,omitempty"` // spells that finish preparing
	Opens       []string `json:"opens,omitempty"`        // buildings here that open
	Closes      []string `json:"closes,omitempty"`       // buildings here that close
}

// WaitPreviewHandler returns, for every wait length the Wait window offers,
// what the wait would change: HP and mana, hunger, effects wearing off or
// setting in, spells finishing preparation, and buildings opening or closing
// here. Fatigue is frozen while waiting, so it's reported once. Read-only.
func WaitPreviewHandler(w http.ResponseWriter, r *http.Request) {
	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		writeQuestError(w, http.StatusBadRequest, "missing npub or save_id")
		return
	}
	sess, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		writeQuestError(w, http.StatusNotFound, "session not found")
		return
	}

	sess.Lock()
	raw, err := json.Marshal(&sess.SaveData)
	queue := append([]types.SpellPrepTask(nil), sess.PrepQueue...)
	sess.Unlock()
	if err != nil {
		writeQuestError(w, http.StatusInternalServerError, "could not read save")
		return
	}
	var now types.SaveFile
	_ = json.Unmarshal(raw, &now)

	database := serverdb.GetDB()
	var names map[string]string
	var openNow map[string]bool
	if database != nil {
		names, _ = building.DistrictBuildingNames(database, now.Location, now.District)
		openNow, _ = building.GetAllBuildingStatesForDistrict(database, now.Location, now.District, now.TimeOfDay)
	}
	visibleBefore := visibleEffects(now.ActiveEffects)

	steps := make([]WaitStep, 0, waitMaxMinutes/waitStepMinutes)
	for m := waitStepMinutes; m <= waitMaxMinutes; m += waitStepMinutes {
		var s types.SaveFile
		_ = json.Unmarshal(raw, &s)

		// The same calls as gametime.HandleWaitAction, in the same order.
		gametime.AdvanceTime(&s, m, false)
		hpLost := now.HP - s.HP
		status.RestoreVitalsForRest(&s, m)

		step := WaitStep{
			Minutes: m, TimeOfDay: s.TimeOfDay, Day: s.CurrentDay,
			HP: s.HP, Mana: s.Mana, Hunger: s.Hunger, HungerName: status.HungerName(s.Hunger),
		}
		if hpLost > 0 {
			step.HPLost = hpLost
		}
		after := visibleEffects(s.ActiveEffects)
		for id, name := range visibleBefore {
			if _, ok := after[id]; !ok {
				step.EffectsEnd = append(step.EffectsEnd, name)
			}
		}
		for id, name := range after {
			if _, ok := visibleBefore[id]; !ok {
				step.EffectsGain = append(step.EffectsGain, name)
			}
		}
		end := spells.AbsoluteMinutes(s.CurrentDay, s.TimeOfDay)
		for _, task := range queue {
			if task.ReadyAtAbsolute <= end {
				step.SpellsReady = append(step.SpellsReady, spells.SpellName(task.SpellID))
			}
		}
		if database != nil && len(openNow) > 0 {
			openThen, _ := building.GetAllBuildingStatesForDistrict(database, s.Location, s.District, s.TimeOfDay)
			for id, was := range openNow {
				if is := openThen[id]; is != was {
					if is {
						step.Opens = append(step.Opens, names[id])
					} else {
						step.Closes = append(step.Closes, names[id])
					}
				}
			}
		}
		steps = append(steps, step)
	}

	// Out on the road, waiting is time spent in the wild: something can find
	// you, and if you haven't stopped you keep walking.
	env := travel.GetEnvironmentData(now.Location)
	// Sleeping till dawn from the Wait window is camping: out in the wild, after
	// 9 PM, with bedding (a bedroll, a blanket or the party's camp set). Inn
	// rooms have their own Sleep button.
	canCamp := env != nil && now.Building == "" && npc.CanSleepNow(now.TimeOfDay) && npc.HasBedding(&now)
	writeQuestJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"hp": now.HP, "max_hp": now.MaxHP, "mana": now.Mana, "max_mana": now.MaxMana,
			"hunger": now.Hunger, "hunger_name": status.HungerName(now.Hunger),
			"fatigue":  now.Fatigue,
			"in_wild":  env != nil,
			"walking":  env != nil && !now.TravelStopped,
			"can_camp": canCamp,
			"steps":    steps,
		},
	})
}

// visibleEffects maps the player-facing active effects (id → name).
func visibleEffects(active []types.ActiveEffect) map[string]string {
	out := map[string]string{}
	for _, ae := range active {
		if data, err := effects.LoadEffectData(ae.EffectID); err == nil && data.Visible {
			out[ae.EffectID] = data.Name
		}
	}
	return out
}
