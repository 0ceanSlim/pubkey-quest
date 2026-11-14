package game

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	gamedata "pubkey-quest/cmd/server/api/data"
	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/mining"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// Mining (docs/draft/mining-system.md). A seam is a POI "mine" node; each has a
// few faces a day (session memory, like picked locks). Working one takes a
// pick, game time and exertion, and a face worked well pays a little XP.

var (
	mineCfgOnce sync.Once
	mineCfg     *types.MiningConfig
)

// miningConfig is mining.json, read once from the systems table.
func miningConfig() *types.MiningConfig {
	mineCfgOnce.Do(func() {
		mineCfg = &types.MiningConfig{}
		database := serverdb.GetDB()
		if database == nil {
			return
		}
		var props string
		if database.QueryRow("SELECT properties FROM systems WHERE id = 'mining'").Scan(&props) != nil {
			return
		}
		if err := json.Unmarshal([]byte(props), mineCfg); err != nil {
			log.Printf("⚠️ mining.json: %v", err)
		}
	})
	return mineCfg
}

// pickTier is the best pick tier you carry (an item's "mining" block), 0 for none.
func pickTier(state *types.SaveFile) int {
	best := 0
	for _, id := range gameutil.CarriedItemIDs(state) {
		item, err := gamedata.LoadItemByID(serverdb.GetDB(), id)
		if err != nil {
			continue
		}
		m, _ := item["mining"].(map[string]any)
		if t, _ := m["tier"].(float64); int(t) > best {
			best = int(t)
		}
	}
	return best
}

func seamMemory(sess *session.GameSession) mining.Memory {
	if sess.Seams == nil {
		sess.Seams = mining.Memory{}
	}
	return sess.Seams
}

// poiMining serves a POI walk's mine nodes.
type poiMining struct{ sess *session.GameSession }

func (p poiMining) seamID(node string) string { return "poi:" + p.sess.ActivePOI.POIID + "/" + node }

func (p poiMining) Seam(s types.Seam) types.Seam { return mining.Effective(miningConfig(), s) }

func (p poiMining) FacesLeft(node string, s types.Seam) int {
	return seamMemory(p.sess).Left(p.seamID(node), p.sess.SaveData.CurrentDay, s.FacesPerDay)
}

func (p poiMining) CantMine(s types.Seam) string {
	if pickTier(&p.sess.SaveData) == 0 {
		return "You'd need a pick to work this rock."
	}
	return ""
}

func (p poiMining) QuickMinutes() int { return miningConfig().Rules.QuickMinutes }

// Quick works one face with a single Athletics roll: the face is used either
// way, time passes, the work tires you, and a success pays the tier's XP.
func (p poiMining) Quick(node string, s types.Seam) poi.MineResult {
	state := &p.sess.SaveData
	cfg := miningConfig()
	tier, _ := mining.Tier(cfg, s.Tier)
	gametime.AdvanceTime(state, cfg.Rules.QuickMinutes, true)
	seamMemory(p.sess).Work(p.seamID(node), state.CurrentDay)
	r := skillcheck.Resolve(buildQuestContext(state).SkillValue("athletics"), s.DC, rand.New(rand.NewSource(time.Now().UnixNano())))
	res := poi.MineResult{Success: r.Success, Lines: []string{poi.RollLine("athletics", r)}}
	if r.Success {
		res.Lines = append(res.Lines, "The rock splits along the vein and the ore comes free.")
		if tier.XP > 0 {
			if adv, err := loadAdvancement(); err == nil {
				xp := character.BonusedXP(state, tier.XP, adv)
				character.GrantXP(state, xp, adv)
				res.Lines = append(res.Lines, fmt.Sprintf("(+%d XP)", xp))
			} else {
				log.Printf("⚠️ mining XP: advancement load failed: %v", err)
			}
		}
	} else {
		res.Lines = append(res.Lines, "You hack at the face for half an hour and break nothing but rock.")
	}
	if n := status.Exert(state, &p.sess.Exertion, tier.Exertion, cfg.Rules.ExertionPerFatigue); n > 0 {
		res.Lines = append(res.Lines, "Your back aches. (+1 fatigue)")
	}
	return res
}
