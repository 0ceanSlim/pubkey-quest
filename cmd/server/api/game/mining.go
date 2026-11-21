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
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gems"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/mining"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// Mining (docs/draft/mining-system.md). A seam is a POI "mine" node; each has a
// few faces a day (session memory, like picked locks). Working one takes a
// pick, game time and exertion, wears the pick, and a face worked well pays a
// little XP. Underground (or at night) you need a light to read the rock. A
// face is worked two ways:
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

// heldPick is the pick you'd work a face with, and where it is.
type heldPick struct {
	mining.Pick
	ID  string
	Ref inventory.SlotRef
}

// bestPick is the best pick you carry or hold (an item's "mining" block: tier,
// swings): the highest tier, then the most swings, then the least worn. Tier 0
// means none.
func bestPick(state *types.SaveFile) heldPick {
	var best heldPick
	inventory.EachGear(state, func(ref inventory.SlotRef, slot map[string]interface{}) {
		id, _ := slot["item"].(string)
		item, err := gamedata.LoadItemByID(serverdb.GetDB(), id)
		if err != nil {
			return
		}
		m, _ := item["mining"].(map[string]any)
		t, _ := m["tier"].(float64)
		sw, _ := m["swings"].(float64)
		if t <= 0 {
			return
		}
		wear, _ := condition.WearTier(slot)
		p := mining.Pick{Name: condition.ItemName(id), Tier: int(t), Swings: int(sw), Wear: int(wear)}
		better := p.Tier > best.Tier ||
			(p.Tier == best.Tier && p.Swings > best.Swings) ||
			(p.Tier == best.Tier && p.Swings == best.Swings && p.Wear < best.Wear)
		if better {
			best = heldPick{Pick: p, ID: id, Ref: ref}
		}
	})
	return best
}

// wearPick wears the pick a face is worked with, and says when it drops a tier.
func wearPick(state *types.SaveFile, pick heldPick, n int) []string {
	if pick.ID == "" || n <= 0 {
		return nil
	}
	slot := inventory.SlotAt(state, pick.Ref)
	if id, _ := slot["item"].(string); id != pick.ID {
		// It moved: wear whichever copy is the best now.
		again := bestPick(state)
		if again.ID != pick.ID {
			return nil
		}
		slot = inventory.SlotAt(state, again.Ref)
	}
	if slot == nil {
		return nil
	}
	before, after := condition.AddWear(slot, n)
	if after > before {
		return []string{fmt.Sprintf("Your %s is %s now.", strings.ToLower(pick.Name), strings.ToLower(after.String()))}
	}
	return nil
}

// timeLines are what passing time had to say (a torch burning out).
func timeLines(msgs []types.EffectMessage) []string {
	var out []string
	for _, m := range msgs {
		if !m.Silent && m.Message != "" {
			out = append(out, m.Message)
		}
	}
	return out
}

func seamMemory(sess *session.GameSession) mining.Memory {
	l := sess.SaveData.Lim()
	if l.Seams == nil {
		l.Seams = map[string]types.SeamDay{}
	}
	return mining.Memory(l.Seams)
}

func mineRng() *rand.Rand { return rand.New(rand.NewSource(time.Now().UnixNano())) }

// useFace counts a face worked at a seam today and charges its exertion. It's
// the "mine-face" a quest's "do" objective asks for.
func useFace(sess *session.GameSession, seamID string, tierN int) []string {
	state := &sess.SaveData
	cfg := miningConfig()
	tier, _ := mining.Tier(cfg, tierN)
	seamMemory(sess).Work(seamID, state.CurrentDay)
	events.Record(state, events.Did, "mine-face", 1)
	if status.Exert(state, &sess.SaveData.Lim().Exertion, tier.Exertion, cfg.Rules.ExertionPerFatigue) > 0 {
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

func (p poiMining) CantMine(_ types.Seam) string {
	if bestPick(&p.sess.SaveData).Tier == 0 {
		return "You'd need a pick to work this rock."
	}
	return ""
}

func (p poiMining) QuickMinutes() int { return miningConfig().Rules.QuickMinutes }

// Quick works one face with a single Athletics roll: the face is used either
// way, time passes, the pick wears, the work tires you, and a success pays the
// tier's XP.
func (p poiMining) Quick(node string, s types.Seam) poi.MineResult {
	state := &p.sess.SaveData
	rules := miningConfig().Rules
	pick := bestPick(state)
	passed := timeLines(gametime.AdvanceTime(state, rules.QuickMinutes, true))
	tired := useFace(p.sess, p.seamID(node), s.Tier)
	tired = append(tired, wearPick(state, pick, rules.QuickWear)...)
	tired = append(tired, passed...)
	r := skillcheck.Resolve(buildQuestContext(state).SkillValue("athletics"), s.DC, mineRng())
	res := poi.MineResult{Success: r.Success, Lines: []string{poi.RollLine("athletics", r)}}
	if r.Success {
		res.Lines = append(res.Lines, "The rock splits along the vein and the ore comes free.")
		res.Lines = append(res.Lines, p.quickGem(s)...)
		res.Lines = append(res.Lines, faceXP(state, s.Tier)...)
	} else {
		res.Lines = append(res.Lines, "You hack at the face for half an hour and break nothing but rock.")
	}
	res.Lines = append(res.Lines, tired...)
	return res
}

// quickGem is a quick roll's chance of a rough gem (gems.json mining): the
// seam's leaning, no bigger than quick_max_size.
func (p poiMining) quickGem(s types.Seam) []string {
	gc := gems.Config()
	rng := mineRng()
	if rng.Float64() >= gc.Mining.QuickChance {
		return nil
	}
	var leaning map[string]int
	if s.Gems != nil {
		leaning = s.Gems.Types
	}
	g, ok := gems.ForFace(gc, s.Tier, leaning, gems.SizeIndex(gc, gc.Mining.QuickMaxSize), rng)
	if !ok {
		return nil
	}
	state := &p.sess.SaveData
	line := fmt.Sprintf("Something glints in the broken rock: a %s!", strings.ToLower(itemDisplayName(g.ID())))
	if added, _ := inventory.AddItemToInventory(state, g.ID(), 1); added == 0 {
		p.sess.Ground.Add(groundSpot(p.sess), g.ID(), 1)
		line += " Your pack is full, so it goes on the ground."
	} else {
		noteAcquired(state, g.ID())
	}
	return []string{line}
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
	pick := bestPick(state)
	ms, err := mining.NewSession(miningConfig(), p.seamID(node), node, sess.ActivePOI.Title, seam,
		pick.Pick, ctx.SkillValue("athletics"), ctx.SkillValue("perception"), mineRng())
	if err != nil {
		return mineRefusal(err.Error())
	}
	ms.Dark = combat.InTheDark(state, seam.Underground)
	sess.MineSession = ms
	lines := []string{fmt.Sprintf("You size up the rock face. %d swings in your arms, %d soundings.", ms.SwingsMax, ms.SoundingsMax)}
	if pick.Wear > 0 {
		lines = append(lines, fmt.Sprintf("Your %s is %s: it costs you swings.", strings.ToLower(pick.Name), strings.ToLower(condition.Tier(pick.Wear).String())))
	}
	if ms.Dark {
		lines = append(lines, "It's too dark to read the rock. Without a light you'll be swinging blind.")
	}
	return &types.GameActionResponse{Success: true, Color: "blue", Message: strings.Join(lines, " "),
		Data: map[string]any{"mine": ms.View(itemDisplayName), "lines": lines}}, nil
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
	ms.Dark = combat.InTheDark(state, ms.Underground)
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
		// A lit light burns while you work (and may burn out).
		lines = append(lines, timeLines(gametime.AdvanceTime(state, out.Minutes, true))...)
	}
	if ms.Worked && !wasWorked {
		lines = append(lines, useFace(sess, ms.SeamID, ms.Tier.Tier)...)
	}
	if out.Wear > 0 {
		lines = append(lines, wearPick(state, bestPick(state), out.Wear)...)
	}
	for _, ore := range sortedKeys(out.Items) {
		n := out.Items[ore]
		added, _ := inventory.AddItemToInventory(state, ore, n)
		if added > 0 {
			noteAcquired(state, ore)
		}
		if added < n {
			sess.Ground.Add(groundSpot(sess), ore, n-added)
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
		kept := ApplyDeath(sess)
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
