package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	gamedata "pubkey-quest/cmd/server/api/data"
	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/mining"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/world"
	"pubkey-quest/types"
)

// Mining (docs/draft/mining-system.md). A seam is a POI "mine" node; each has a
// few faces a day (session memory, like picked locks). Working one takes a
// pick, game time and exertion, and a face worked well pays a little XP. A face
// is worked two ways:
//
//   - the node's "#quick": one Athletics roll for the seam's quick loot
//   - mine_begin + mine_act: the Veins minigame (mining.Session)

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

// bestPick is the best pick you carry (an item's "mining" block: tier, swings);
// tier 0 means none.
func bestPick(state *types.SaveFile) mining.Pick {
	var best mining.Pick
	for _, id := range gameutil.CarriedItemIDs(state) {
		item, err := gamedata.LoadItemByID(serverdb.GetDB(), id)
		if err != nil {
			continue
		}
		m, _ := item["mining"].(map[string]any)
		t, _ := m["tier"].(float64)
		sw, _ := m["swings"].(float64)
		if p := (mining.Pick{Tier: int(t), Swings: int(sw)}); p.Tier > best.Tier || (p.Tier == best.Tier && p.Swings > best.Swings) {
			best = p
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

func mineRng() *rand.Rand { return rand.New(rand.NewSource(time.Now().UnixNano())) }

// useFace counts a face worked at a seam today and charges its exertion.
func useFace(sess *session.GameSession, seamID string, tierN int) []string {
	state := &sess.SaveData
	cfg := miningConfig()
	tier, _ := mining.Tier(cfg, tierN)
	seamMemory(sess).Work(seamID, state.CurrentDay)
	if status.Exert(state, &sess.Exertion, tier.Exertion, cfg.Rules.ExertionPerFatigue) > 0 {
		return []string{"Your back aches. (+1 fatigue)"}
	}
	return nil
}

// faceXP pays a face worked well its tier's small XP.
func faceXP(state *types.SaveFile, tierN int) []string {
	tier, _ := mining.Tier(miningConfig(), tierN)
	if tier.XP <= 0 {
		return nil
	}
	adv, err := loadAdvancement()
	if err != nil {
		log.Printf("⚠️ mining XP: advancement load failed: %v", err)
		return nil
	}
	xp := character.BonusedXP(state, tier.XP, adv)
	character.GrantXP(state, xp, adv)
	return []string{fmt.Sprintf("(+%d XP)", xp)}
}

// poiMining serves a POI walk's mine nodes.
type poiMining struct{ sess *session.GameSession }

func (p poiMining) seamID(node string) string { return "poi:" + p.sess.ActivePOI.POIID + "/" + node }

func (p poiMining) Seam(s types.Seam) types.Seam { return mining.Effective(miningConfig(), s) }

func (p poiMining) FacesLeft(node string, s types.Seam) int {
	return seamMemory(p.sess).Left(p.seamID(node), p.sess.SaveData.CurrentDay, s.FacesPerDay)
}

func (p poiMining) CantMine(s types.Seam) string {
	if bestPick(&p.sess.SaveData).Tier == 0 {
		return "You'd need a pick to work this rock."
	}
	return ""
}

func (p poiMining) QuickMinutes() int { return miningConfig().Rules.QuickMinutes }

// Quick works one face with a single Athletics roll: the face is used either
// way, time passes, the work tires you, and a success pays the tier's XP.
func (p poiMining) Quick(node string, s types.Seam) poi.MineResult {
	state := &p.sess.SaveData
	gametime.AdvanceTime(state, miningConfig().Rules.QuickMinutes, true)
	tired := useFace(p.sess, p.seamID(node), s.Tier)
	r := skillcheck.Resolve(buildQuestContext(state).SkillValue("athletics"), s.DC, mineRng())
	res := poi.MineResult{Success: r.Success, Lines: []string{poi.RollLine("athletics", r)}}
	if r.Success {
		res.Lines = append(res.Lines, "The rock splits along the vein and the ore comes free.")
		res.Lines = append(res.Lines, faceXP(state, s.Tier)...)
	} else {
		res.Lines = append(res.Lines, "You hack at the face for half an hour and break nothing but rock.")
	}
	res.Lines = append(res.Lines, tired...)
	return res
}

// ─── Veins: the face as a minigame ───────────────────────────────────────────

func mineRefusal(msg string) (*types.GameActionResponse, error) {
	return &types.GameActionResponse{Success: false, Message: msg, Color: "yellow"}, nil
}

// handleMineBegin lays out a face at the walk's current mine node.
// params: node
func handleMineBegin(sess *session.GameSession, params map[string]any) (*types.GameActionResponse, error) {
	state := &sess.SaveData
	node, _ := params["node"].(string)
	if sess.ActivePOI == nil || node == "" || poi.BaseNode(sess.ActivePOI.CurrentNode) != node {
		return mineRefusal("You're not at that rock face.")
	}
	n, ok := sess.ActivePOI.Nodes[node]
	if !ok || n.Type != types.POIStepMine || n.Seam == nil {
		return mineRefusal("There's no rock face there.")
	}
	p := poiMining{sess}
	seam := p.Seam(*n.Seam)
	if why := p.CantMine(seam); why != "" {
		return mineRefusal(why)
	}
	if p.FacesLeft(node, seam) == 0 {
		return mineRefusal("The face is worked out. Come back tomorrow.")
	}
	ctx := buildQuestContext(state)
	ms, err := mining.NewSession(miningConfig(), p.seamID(node), node, sess.ActivePOI.Title, seam,
		bestPick(state), ctx.SkillValue("athletics"), ctx.SkillValue("perception"), mineRng())
	if err != nil {
		return mineRefusal(err.Error())
	}
	sess.MineSession = ms
	msg := fmt.Sprintf("You size up the rock face. %d swings in your arms, %d soundings.", ms.SwingsMax, ms.SoundingsMax)
	return &types.GameActionResponse{Success: true, Color: "blue", Message: msg,
		Data: map[string]any{"mine": ms.View(itemDisplayName), "lines": []string{msg}}}, nil
}

// handleMineAct resolves one move and applies its consequences once: time, the
// face counted (and its exertion) on the first move, ore into the pack, cave-in
// damage, and XP when a face that gave ore ends without coming down.
// params: session_id, version, act ("sound"|"strike"|"stop"), cell
func handleMineAct(sess *session.GameSession, params map[string]any) (*types.GameActionResponse, error) {
	state := &sess.SaveData
	ms := sess.MineSession
	id, _ := params["session_id"].(string)
	if ms == nil || id == "" || id != ms.ID {
		return mineRefusal("You're not working that face any more.")
	}
	act, _ := params["act"].(string)
	wasWorked := ms.Worked
	out, err := ms.Act(mining.Action{Kind: act, Cell: paramIntOr(params["cell"], -1)}, paramIntOr(params["version"], -1), itemDisplayName)
	data := map[string]any{}
	if errors.Is(err, mining.ErrStale) {
		// Already handled (a double tap, a retried request): send the current
		// face back and change nothing.
		data["mine"], data["stale"] = ms.View(itemDisplayName), true
		return &types.GameActionResponse{Success: true, Data: data}, nil
	}
	if err != nil {
		return mineRefusal(err.Error())
	}

	lines := out.Lines
	if out.Minutes > 0 {
		gametime.AdvanceTime(state, out.Minutes, true)
	}
	if ms.Worked && !wasWorked {
		lines = append(lines, useFace(sess, ms.SeamID, ms.Tier.Tier)...)
	}
	for _, ore := range sortedKeys(out.Items) {
		n := out.Items[ore]
		added, _ := inventory.AddItemToInventory(state, ore, n)
		if added > 0 {
			noteAcquired(state, ore)
		}
		if added < n {
			sess.Ground.Add(world.GroundKey(state), ore, n-added)
			lines = append(lines, "Your pack is full, so it goes on the ground.")
		}
		ms.Gained += n
	}
	if out.Damage > 0 {
		state.HP = max(state.HP-out.Damage, 0)
	}
	if out.Ended && ms.Status != mining.StatusCollapsed && ms.Gained > 0 {
		lines = append(lines, faceXP(state, ms.Tier.Tier)...)
	}
	data["mine"] = ms.View(itemDisplayName)

	switch {
	case state.HP <= 0:
		// A cave-in can kill: the same death flow as a POI damage node.
		sess.MineSession = nil
		kept := ApplyDeath(state)
		sess.ActivePOI = nil
		lines = append(lines, fmt.Sprintf("You have fallen. You wake in %s, stripped of your belongings — but your experience endures.", state.Location))
		data["death"] = map[string]any{"outcome": "defeat", "location": state.Location, "loot_kept": kept}
	case out.Ended:
		sess.MineSession = nil
		if sess.ActivePOI != nil {
			if res, err := stepPOI(sess, ms.Node, data); err == nil {
				data["step"] = res
			}
		}
	}
	data["lines"] = lines
	color := "yellow"
	switch {
	case len(out.Items) > 0:
		color = "green"
	case out.Damage > 0:
		color = "red"
	}
	return &types.GameActionResponse{Success: true, Message: strings.Join(lines, " "), Color: color, Data: data}, nil
}
