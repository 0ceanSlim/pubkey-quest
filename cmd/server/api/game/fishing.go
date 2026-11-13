package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	gamedata "pubkey-quest/cmd/server/api/data"
	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/fishing"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/social"
	"pubkey-quest/cmd/server/world"
	"pubkey-quest/types"
)

// ─── Fishing (docs/draft/fishing-system.md) ──────────────────────────────────
//
// A spot is fished from a town district (fishing.json "district") or a POI's
// fishing node. Everything goes through game actions, under the session lock:
//
//   - fish_open: what you can do here (rod, baits, bare hands, the net, worms)
//   - fish_begin + fish_act: Cast & Reel (fishing.Session), versioned like lock_act
//   - fish_quick: one Survival roll for a fish of medium or smaller
//   - haul_net / dig_worms: the net in the shallows, worms from the bank
//   - cut_bait: a tiny or small fish into 2 cut bait
//
// Any other action walks away from the water. Time passes as you fish (fatigue
// runs as normal), and each spot bites less once it's been fished hard today.

var (
	fishCfgOnce sync.Once
	fishCfg     *types.FishingConfig
)

// fishingConfig is fishing.json, read once from the systems table.
func fishingConfig() *types.FishingConfig {
	fishCfgOnce.Do(func() {
		fishCfg = &types.FishingConfig{}
		database := serverdb.GetDB()
		if database == nil {
			return
		}
		var props string
		if database.QueryRow("SELECT properties FROM systems WHERE id = 'fishing'").Scan(&props) != nil {
			return
		}
		if err := json.Unmarshal([]byte(props), fishCfg); err != nil {
			log.Printf("⚠️ fishing.json: %v", err)
		}
	})
	return fishCfg
}

func fishRng() *rand.Rand { return rand.New(rand.NewSource(time.Now().UnixNano())) }

func init() { quest.FishMatches = fishMatches }

// fishMatches: does a landed fish meet a catch objective? Target is "any", a
// species, or "water-<water>"; minSize ("" = any) is the smallest size that counts.
func fishMatches(target, minSize, itemID string) bool {
	f := netFish(itemID)
	if f == nil {
		return false
	}
	cfg := fishingConfig()
	if minSize != "" && fishing.SizeIndex(cfg, f.Size) < fishing.SizeIndex(cfg, minSize) {
		return false
	}
	switch {
	case target == "" || target == "any":
		return true
	case strings.HasPrefix(target, "water-"):
		return slices.Contains(cfg.Species[f.Species].Water, strings.TrimPrefix(target, "water-"))
	}
	return f.Species == target
}

// barbarianFishing is the side quest that teaches barehand fishing; while it's
// underway, its spot allows it for the lesson.
const (
	barbarianFishing = "barbarian-fishing"
	barbarianSpot    = "cragspire-falls"
)

// townFishingSpot is the spot in the district you're standing in, outdoors in
// town ("" when there's none).
func townFishingSpot(state *types.SaveFile) string {
	if state.Building != "" || travel.GetEnvironmentData(state.Location) != nil {
		return ""
	}
	// A save names its district by key ("east"); spots by id ("goldenhaven-east").
	here := state.Location + "-" + state.District
	for id, spot := range fishingConfig().Spots {
		if spot.District != "" && (spot.District == here || spot.District == state.District) {
			return id
		}
	}
	return ""
}

// fishingSpotAt resolves where a request says you're fishing: a town district
// or the fishing node of your current walk.
func fishingSpotAt(sess *session.GameSession, kind, node string) (string, string) {
	switch kind {
	case "town":
		if id := townFishingSpot(&sess.SaveData); id != "" {
			return id, ""
		}
	case "poi":
		if sess.ActivePOI == nil || node == "" || poi.BaseNode(sess.ActivePOI.CurrentNode) != node {
			break
		}
		if n, ok := sess.ActivePOI.Nodes[node]; ok && n.Type == types.POIStepFishing {
			if _, ok := fishingConfig().Spots[n.Spot]; ok {
				return n.Spot, ""
			}
		}
	}
	return "", "There's no fishing here."
}

// itemFishing is an item's "fishing" block (rods, bait, lures), or nil.
func itemFishing(itemID string) map[string]any {
	item, err := gamedata.LoadItemByID(serverdb.GetDB(), itemID)
	if err != nil {
		return nil
	}
	f, _ := item["fishing"].(map[string]any)
	return f
}

func itemDisplayName(itemID string) string {
	if it, err := serverdb.GetItemByID(itemID); err == nil && it != nil {
		return it.Name
	}
	return strings.ReplaceAll(itemID, "-", " ")
}

// bestRod is the strongest rod you carry (by line), and its name.
func bestRod(state *types.SaveFile) (fishing.Gear, string, bool) {
	var best fishing.Gear
	name := ""
	for _, id := range gameutil.CarriedItemIDs(state) {
		f := itemFishing(id)
		line, _ := f["line"].(float64)
		if int(line) <= best.Line {
			continue
		}
		cast, _ := f["cast"].(float64)
		tell, _ := f["tell"].(float64)
		best, name = fishing.Gear{Line: int(line), Cast: int(cast), Tell: int(tell)}, itemDisplayName(id)
	}
	return best, name, best.Line > 0
}

// baitKind is an item's bait kind ("" if it isn't bait).
func baitKind(itemID string) string {
	k, _ := itemFishing(itemID)["bait"].(string)
	return k
}

// carriedBaits lists the bait and lures you carry, for the window.
func carriedBaits(state *types.SaveFile) []map[string]any {
	var out []map[string]any
	seen := map[string]bool{}
	for _, id := range gameutil.CarriedItemIDs(state) {
		kind := baitKind(id)
		if kind == "" || seen[id] {
			continue
		}
		seen[id] = true
		b := fishingConfig().Baits[kind]
		out = append(out, map[string]any{
			"item_id": id, "name": itemDisplayName(id), "kind": kind,
			"count": gameutil.CountItem(state, id), "lure": b.Lure,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) < out[j]["name"].(string) })
	return out
}

// barehandAllowed: after Barbarian Fishing anywhere, and during it at the falls.
func barehandAllowed(state *types.SaveFile, spotID string) bool {
	if slices.Contains(state.QuestsCompleted, barbarianFishing) {
		return true
	}
	if spotID != barbarianSpot {
		return false
	}
	return slices.ContainsFunc(state.QuestsActive, func(q types.QuestProgress) bool { return q.QuestID == barbarianFishing })
}

func fishMemory(sess *session.GameSession) fishing.Memory {
	if sess.FishMemory == nil {
		sess.FishMemory = fishing.Memory{}
	}
	return sess.FishMemory
}

// fishConditions is the world around a cast right now.
func fishConditions(sess *session.GameSession, spotID, bait string, barehand bool) fishing.Conditions {
	state := &sess.SaveData
	ctx := buildQuestContext(state)
	spot := fishingConfig().Spots[spotID]
	return fishing.Conditions{
		TimeOfDay: state.TimeOfDay,
		Day:       gametime.Config().SunlightHours,
		Survival:  ctx.SkillValue("survival"),
		Athletics: ctx.SkillValue("athletics"),
		Bait:      bait,
		Barehand:  barehand,
		Dark:      combat.BlindAtNight(state),
		Party:     social.PartyOf(sess.Npub) != nil,
		Depletion: fishMemory(sess).Factor(spotID, state.CurrentDay, spot.DailyCatch),
	}
}

// fishWindow is what the fishing window offers at a spot.
func fishWindow(sess *session.GameSession, spotID string) map[string]any {
	state := &sess.SaveData
	spot := fishingConfig().Spots[spotID]
	rod, rodName, hasRod := bestRod(state)
	w := map[string]any{
		"spot": spot.Name, "spot_id": spotID, "water": spot.Water,
		"baits":      carriedBaits(state),
		"barehand":   barehandAllowed(state, spotID),
		"net":        spot.Shallows && gameutil.PlayerHasItem(state, "net"),
		"worms":      spot.Worms,
		"fished_out": fishMemory(sess).Factor(spotID, state.CurrentDay, spot.DailyCatch) == 0,
		"dark":       combat.BlindAtNight(state),
	}
	if hasRod {
		w["rod"] = map[string]any{"name": rodName, "line": rod.Line}
	}
	return w
}

// cantFish says why a spot can't be fished at all right now ("" when it can).
func cantFish(sess *session.GameSession, spotID string) string {
	w := fishWindow(sess, spotID)
	if w["rod"] == nil && w["barehand"] != true && w["net"] != true && w["worms"] != true {
		return "You'd need a fishing rod to fish here."
	}
	return ""
}

// poiFishing serves the walker's fishing nodes.
type poiFishing struct{ sess *session.GameSession }

func (p poiFishing) SpotName(spot string) string {
	if s, ok := fishingConfig().Spots[spot]; ok {
		return s.Name
	}
	return "here"
}
func (p poiFishing) CantFish(spot string) string { return cantFish(p.sess, spot) }

func fishRefusal(msg string) *GameActionResponse {
	return &GameActionResponse{Success: false, Message: msg, Color: "yellow"}
}

// handleFishOpen describes the fishing at a spot for the window.
// params: kind ("town"|"poi"), node
func handleFishOpen(sess *session.GameSession, params map[string]any) (*GameActionResponse, error) {
	kind, _ := params["kind"].(string)
	node, _ := params["node"].(string)
	spotID, why := fishingSpotAt(sess, kind, node)
	if spotID == "" {
		return fishRefusal(why), nil
	}
	return &GameActionResponse{Success: true, Data: map[string]any{"window": fishWindow(sess, spotID)}}, nil
}

// chooseBait checks the bait item a request names ("" = bare hook).
func chooseBait(state *types.SaveFile, itemID string) (string, string) {
	if itemID == "" {
		return "", ""
	}
	if gameutil.CountItem(state, itemID) <= 0 {
		return "", fmt.Sprintf("You don't have any %s.", itemDisplayName(itemID))
	}
	kind := baitKind(itemID)
	if kind == "" {
		return "", fmt.Sprintf("You can't fish with %s.", itemDisplayName(itemID))
	}
	return kind, ""
}

// handleFishBegin starts Cast & Reel (or barehand fishing).
// params: kind, node, bait (item id or ""), barehand (bool)
func handleFishBegin(sess *session.GameSession, params map[string]any) (*GameActionResponse, error) {
	state := &sess.SaveData
	kind, _ := params["kind"].(string)
	node, _ := params["node"].(string)
	baitItem, _ := params["bait"].(string)
	barehand, _ := params["barehand"].(bool)
	spotID, why := fishingSpotAt(sess, kind, node)
	if spotID == "" {
		return fishRefusal(why), nil
	}
	cfg := fishingConfig()
	if fishMemory(sess).Factor(spotID, state.CurrentDay, cfg.Spots[spotID].DailyCatch) == 0 {
		return fishRefusal("This water's fished out for today. Try again tomorrow, or somewhere else."), nil
	}
	var gear fishing.Gear
	if barehand {
		if !barehandAllowed(state, spotID) {
			return fishRefusal("You don't know how to catch fish with your bare hands."), nil
		}
		baitItem = ""
	} else {
		var ok bool
		if gear, _, ok = bestRod(state); !ok {
			return fishRefusal("You need a fishing rod."), nil
		}
	}
	bait, why := chooseBait(state, baitItem)
	if why != "" {
		return fishRefusal(why), nil
	}
	fs, err := fishing.NewSession(cfg, spotID, gear, fishConditions(sess, spotID, bait, barehand), fishRng())
	if err != nil {
		return fishRefusal(err.Error()), nil
	}
	fs.Kind, fs.Node, fs.BaitItem = kind, node, baitItem
	sess.FishSession = fs
	gametime.AdvanceTime(state, cfg.Rules.RecastMinute, true)
	msg := "You bait the hook and cast out."
	switch {
	case barehand:
		msg = "You roll up your sleeves and wade in, watching the water."
	case bait == "":
		msg = "You cast a bare hook. Something might be hungry enough."
	}
	return &GameActionResponse{Success: true, Message: msg, Color: "blue", Data: map[string]any{
		"fish": fs.View(), "window": fishWindow(sess, spotID), "lines": []string{msg},
	}}, nil
}

// handleFishAct resolves one move and applies what it did.
// params: session_id, version, act, bait (on recast: the bait item for the next cast)
func handleFishAct(sess *session.GameSession, params map[string]any) (*GameActionResponse, error) {
	state := &sess.SaveData
	fs := sess.FishSession
	id, _ := params["session_id"].(string)
	if fs == nil || id == "" || id != fs.ID {
		return fishRefusal("You're not fishing any more."), nil
	}
	act, _ := params["act"].(string)
	version := paramIntOr(params["version"], -1)
	data := map[string]any{}

	// The world moves on while you fish: the hour, the light, how fished-out it is.
	fresh := fishConditions(sess, fs.SpotID, fs.Cond.Bait, fs.Cond.Barehand)
	if act == "recast" {
		if raw, ok := params["bait"].(string); ok && raw != fs.BaitItem {
			kind, why := chooseBait(state, raw)
			if why != "" {
				return fishRefusal(why), nil
			}
			fs.BaitItem, fresh.Bait = raw, kind
		}
		if fresh.Depletion == 0 {
			return fishRefusal("This water's fished out for today."), nil
		}
	}
	fs.Cond = fresh

	out, err := fs.Act(act, version)
	if errors.Is(err, fishing.ErrStale) {
		data["fish"], data["stale"] = fs.View(), true
		return &GameActionResponse{Success: true, Data: data}, nil
	}
	if err != nil {
		return fishRefusal(err.Error()), nil
	}
	lines := applyFishOutcome(sess, fs, out, data)
	return finishFishAct(sess, fs, lines, data), nil
}

// applyFishOutcome applies a move's consequences once: time, bait, the catch,
// a find, a monster on the line.
func applyFishOutcome(sess *session.GameSession, fs *fishing.Session, out fishing.Outcome, data map[string]any) []string {
	state := &sess.SaveData
	lines := out.Lines
	if out.Minutes > 0 {
		gametime.AdvanceTime(state, out.Minutes, true)
	}
	lure := fishingConfig().Baits[fs.Cond.Bait].Lure
	if fs.BaitItem != "" && ((out.BaitUsed && !lure) || out.LureLost) {
		gameutil.ConsumeItem(state, fs.BaitItem, 1)
		if gameutil.CountItem(state, fs.BaitItem) == 0 {
			lines = append(lines, fmt.Sprintf("That was your last %s. You're on a bare hook now.", strings.ToLower(itemDisplayName(fs.BaitItem))))
			fs.BaitItem = ""
			fs.SetBait("")
		}
	}
	if out.Landed != nil {
		lines = append(lines, addCatch(sess, *out.Landed, fs.SpotID)...)
	}
	if out.Find != "" {
		lines = append(lines, addFind(sess, out.Find, out.FindQty)...)
	}
	if out.Monster != "" {
		if err := startFishingFight(sess, fs, out.Monster); err != nil {
			log.Printf("⚠️ fishing monster %q: %v", out.Monster, err)
			lines = append(lines, "...but it tears free and is gone.")
		} else {
			buildCombatPayload(sess, data)
		}
	}
	return lines
}

// addCatch puts a landed fish in the pack with its weight (or on the ground if
// the pack's full), counts it toward the spot's day, and records it.
func addCatch(sess *session.GameSession, f fishing.Fish, spotID string) []string {
	state := &sess.SaveData
	id := f.ItemID()
	st := condition.NewState(id, condition.Now(state))
	if st == nil {
		st = map[string]any{}
	}
	st[condition.KeyLb] = f.Lb
	fishMemory(sess).Landed(spotID, state.CurrentDay)
	events.Record(state, events.FishCaught, id, 1)
	if added, _ := inventory.AddItemWithState(state, id, 1, st); added > 0 {
		noteAcquired(state, id)
		return nil
	}
	sess.Ground.AddWithState(world.GroundKey(state), id, 1, st)
	return []string{"Your pack is full, so you lay it on the bank."}
}

// addFind adds something fished up that isn't a fish.
func addFind(sess *session.GameSession, itemID string, qty int) []string {
	state := &sess.SaveData
	if itemID == "gold-piece" {
		_ = gameutil.AddGoldToInventory(state.Inventory, qty)
		return []string{fmt.Sprintf("You reel in a sodden purse: %d gold!", qty)}
	}
	name := itemDisplayName(itemID)
	line := fmt.Sprintf("You reel in... %s.", aOrAn(name))
	if itemID == "torch" {
		line = "You reel in a dry branch, just right for a torch."
	}
	if added, _ := inventory.AddItemToInventory(state, itemID, qty); added > 0 {
		noteAcquired(state, itemID)
	} else {
		sess.Ground.Add(world.GroundKey(state), itemID, qty)
		line += " Your pack is full, so you leave it on the bank."
	}
	return []string{line}
}

func aOrAn(name string) string {
	lower := strings.ToLower(name)
	if lower == "" {
		return name
	}
	if strings.ContainsRune("aeiou", rune(lower[0])) {
		return "an " + lower
	}
	return "a " + lower
}

// startFishingFight starts combat with whatever took the bait. In a walk, the
// walk resumes at the fishing node after the fight.
func startFishingFight(sess *session.GameSession, fs *fishing.Session, monster string) error {
	state := &sess.SaveData
	advancement, err := loadAdvancement()
	if err != nil {
		return err
	}
	cs, err := combat.StartEncounter(serverdb.GetDB(), state, sess.Npub, combat.EncounterSpec{
		MonsterIDs: []string{monster}, EnvironmentID: state.Location,
	}, advancement)
	if err != nil {
		return err
	}
	sess.ActiveCombat = cs
	if fs.Kind == "poi" && sess.ActivePOI != nil {
		sess.ActivePOI.ResumeNext = fs.Node
	}
	return nil
}

// finishFishAct builds the response and, when the session's over, closes it
// (re-presenting a POI's fishing node).
func finishFishAct(sess *session.GameSession, fs *fishing.Session, lines []string, data map[string]any) *GameActionResponse {
	data["fish"] = fs.View()
	data["lines"] = lines
	data["window"] = fishWindow(sess, fs.SpotID)
	if fs.Phase == fishing.PhaseOver {
		sess.FishSession = nil
		if fs.Kind == "poi" && sess.ActivePOI != nil && sess.ActiveCombat == nil {
			if res, err := stepPOI(sess, fs.Node, data); err == nil {
				data["step"] = res
			}
		}
	}
	color := "blue"
	if v := fs.View(); v["catch"] != nil {
		color = "green"
	}
	return &GameActionResponse{Success: true, Message: strings.Join(lines, " "), Color: color, Data: data}
}

// handleFishQuick is the quick roll. params: kind, node, bait
func handleFishQuick(sess *session.GameSession, params map[string]any) (*GameActionResponse, error) {
	state := &sess.SaveData
	kind, _ := params["kind"].(string)
	node, _ := params["node"].(string)
	baitItem, _ := params["bait"].(string)
	spotID, why := fishingSpotAt(sess, kind, node)
	if spotID == "" {
		return fishRefusal(why), nil
	}
	if _, _, ok := bestRod(state); !ok {
		return fishRefusal("You need a fishing rod."), nil
	}
	bait, why := chooseBait(state, baitItem)
	if why != "" {
		return fishRefusal(why), nil
	}
	c := fishConditions(sess, spotID, bait, false)
	if c.Depletion == 0 {
		return fishRefusal("This water's fished out for today."), nil
	}
	cfg := fishingConfig()
	r := fishing.Quick(cfg, cfg.Spots[spotID], c, fishRng())
	gametime.AdvanceTime(state, r.Minutes, true)
	if r.BaitUsed && baitItem != "" {
		gameutil.ConsumeItem(state, baitItem, 1)
	}
	check := fmt.Sprintf("(Survival %d vs %d)", r.Check.Total, r.Check.DC)
	if r.Fish == nil {
		msg := "You fish for a while, but nothing bites. " + check
		return &GameActionResponse{Success: true, Message: msg, Color: "yellow", Data: map[string]any{"lines": []string{msg}, "window": fishWindow(sess, spotID)}}, nil
	}
	sp, size := cfg.Species[r.Fish.Species], r.Fish.Size
	lines := []string{fmt.Sprintf("You land a %s %s, %s. %s", size, strings.ToLower(sp.Name), fishing.WeighOut(r.Fish.Lb), check)}
	lines = append(lines, addCatch(sess, *r.Fish, spotID)...)
	return &GameActionResponse{Success: true, Message: strings.Join(lines, " "), Color: "green", Data: map[string]any{"lines": lines, "window": fishWindow(sess, spotID)}}, nil
}

// handleHaulNet drags the net through the shallows. params: kind, node
func handleHaulNet(sess *session.GameSession, params map[string]any) (*GameActionResponse, error) {
	state := &sess.SaveData
	kind, _ := params["kind"].(string)
	node, _ := params["node"].(string)
	spotID, why := fishingSpotAt(sess, kind, node)
	if spotID == "" {
		return fishRefusal(why), nil
	}
	cfg := fishingConfig()
	spot := cfg.Spots[spotID]
	if !spot.Shallows {
		return fishRefusal("It's too deep here to haul a net."), nil
	}
	if !gameutil.PlayerHasItem(state, "net") {
		return fishRefusal("You need a net."), nil
	}
	r := fishing.Haul(cfg, spot, fishConditions(sess, spotID, "", false), fishRng())
	gametime.AdvanceTime(state, r.Minutes, true)
	check := fmt.Sprintf("(Survival %d vs %d)", r.Check.Total, r.Check.DC)
	if len(r.Items) == 0 {
		msg := "You drag the net through the shallows and haul up weed and mud. " + check
		return &GameActionResponse{Success: true, Message: msg, Color: "yellow", Data: map[string]any{"lines": []string{msg}, "window": fishWindow(sess, spotID)}}, nil
	}
	lines := []string{"You drag the net through the shallows and haul it in. " + check}
	for _, id := range sortedKeys(r.Items) {
		n := r.Items[id]
		lines = append(lines, fmt.Sprintf("  %d × %s", n, itemDisplayName(id)))
		for range n {
			if strings.HasPrefix(id, "fish-") {
				if f := netFish(id); f != nil {
					lines = append(lines, addCatch(sess, *f, spotID)...)
					continue
				}
			}
			if added, _ := inventory.AddItemToInventory(state, id, 1); added > 0 {
				noteAcquired(state, id)
			} else {
				sess.Ground.Add(world.GroundKey(state), id, 1)
				lines = append(lines, "Your pack is full, so you leave it on the bank.")
			}
		}
	}
	return &GameActionResponse{Success: true, Message: strings.Join(lines, " "), Color: "green", Data: map[string]any{"lines": lines, "window": fishWindow(sess, spotID)}}, nil
}

// netFish turns a netted fish's item id back into a fish with a weight in its band.
func netFish(itemID string) *fishing.Fish {
	cfg := fishingConfig()
	for sid, sp := range cfg.Species {
		for i, size := range cfg.Sizes {
			if types.FishItemID(sid, size.ID) == itemID {
				return &fishing.Fish{Species: sid, Size: size.ID, Lb: fishing.FishWeight(cfg, sp, i)}
			}
		}
	}
	return nil
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// handleDigWorms turns the bank over for worms. params: kind, node
func handleDigWorms(sess *session.GameSession, params map[string]any) (*GameActionResponse, error) {
	state := &sess.SaveData
	kind, _ := params["kind"].(string)
	node, _ := params["node"].(string)
	spotID, why := fishingSpotAt(sess, kind, node)
	if spotID == "" {
		return fishRefusal(why), nil
	}
	cfg := fishingConfig()
	if !cfg.Spots[spotID].Worms {
		return fishRefusal("The ground here's no good for worms."), nil
	}
	r := fishing.DigWorms(cfg, fishConditions(sess, spotID, "", false), fishRng())
	gametime.AdvanceTime(state, r.Minutes, true)
	check := fmt.Sprintf("(Survival %d vs %d)", r.Check.Total, r.Check.DC)
	n := r.Items["worms"]
	if n == 0 {
		msg := "You turn over the soft earth by the water, but find nothing. " + check
		return &GameActionResponse{Success: true, Message: msg, Color: "yellow", Data: map[string]any{"lines": []string{msg}, "window": fishWindow(sess, spotID)}}, nil
	}
	added, _ := inventory.AddItemToInventory(state, "worms", n)
	msg := fmt.Sprintf("You dig up %d fat worms. %s", added, check)
	if added < n {
		msg += " Your pack's full, so you leave the rest."
	}
	return &GameActionResponse{Success: true, Message: msg, Color: "green", Data: map[string]any{"lines": []string{msg}, "window": fishWindow(sess, spotID)}}, nil
}

// handleCutBait cuts a tiny or small fish into 2 cut bait. params: item_id
func handleCutBait(state *types.SaveFile, params map[string]any) (*GameActionResponse, error) {
	id, _ := params["item_id"].(string)
	f := netFish(id)
	cfg := fishingConfig()
	if f == nil || fishing.SizeIndex(cfg, f.Size) > fishing.SizeIndex(cfg, "small") {
		return fishRefusal("Only a tiny or small fish can be cut into bait."), nil
	}
	if !gameutil.ConsumeItem(state, id, 1) {
		return fishRefusal(fmt.Sprintf("You don't have a %s.", strings.ToLower(itemDisplayName(id)))), nil
	}
	added, _ := inventory.AddItemToInventory(state, "cut-bait", 2)
	msg := fmt.Sprintf("You cut the %s into %d strips of bait.", strings.ToLower(itemDisplayName(id)), added)
	return &GameActionResponse{Success: true, Message: msg, Color: "blue"}, nil
}

// fishActions are the fishing game actions; any other abandons the session.
var fishActions = map[string]bool{"fish_open": true, "fish_begin": true, "fish_act": true}

// routeFishing handles the fishing actions (false: not one of them).
func routeFishing(sess *session.GameSession, action GameAction) (*GameActionResponse, bool, error) {
	var resp *GameActionResponse
	var err error
	switch action.Type {
	case "fish_open":
		resp, err = handleFishOpen(sess, action.Params)
	case "fish_begin":
		resp, err = handleFishBegin(sess, action.Params)
	case "fish_act":
		resp, err = handleFishAct(sess, action.Params)
	case "fish_quick":
		resp, err = handleFishQuick(sess, action.Params)
	case "haul_net":
		resp, err = handleHaulNet(sess, action.Params)
	case "dig_worms":
		resp, err = handleDigWorms(sess, action.Params)
	case "cut_bait":
		resp, err = handleCutBait(&sess.SaveData, action.Params)
	default:
		return nil, false, nil
	}
	return resp, true, err
}

// ─── Finn's trophies ─────────────────────────────────────────────────────────

// Finn Brackwater mounts a huge fish as a trophy for a fee (fishing.json
// "trophy"): it stops spoiling and sells for more. His menu gets the option
// while he's in the shop and you carry a huge fish.
const (
	trophyMounter = "tackle-keeper"
	mountChoice   = "fish:mount"
)

// hugeFish is every huge fish you carry, by item id (with how many).
func hugeFish(state *types.SaveFile) map[string]int {
	out := map[string]int{}
	for _, id := range gameutil.CarriedItemIDs(state) {
		if f := netFish(id); f != nil && f.Size == "huge" {
			out[id] = gameutil.CountItem(state, id)
		}
	}
	return out
}

// mountFee is what Finn charges to mount one.
func mountFee(itemID string) int {
	cfg := fishingConfig()
	f := netFish(itemID)
	if f == nil {
		return 0
	}
	size := cfg.Sizes[fishing.SizeIndex(cfg, f.Size)]
	return max(1, int(float64(fishing.FishValue(cfg, cfg.Species[f.Species], size))*cfg.Trophy.Fee))
}

// injectMountOption adds "Mount my huge fish" to Finn's menu, before goodbye.
func injectMountOption(resp *types.GameActionResponse, npcID string, state *types.SaveFile) {
	if npcID != trophyMounter || resp == nil || resp.Delta == nil || !npc.OnDuty(state, npcID, "open_shop") {
		return
	}
	fish := hugeFish(state)
	if len(fish) == 0 {
		return
	}
	dlg, ok := resp.Delta["npc_dialogue"].(map[string]interface{})
	if !ok {
		return
	}
	npcData, err := serverdb.GetNPCByID(npcID)
	if err != nil || npcData == nil {
		return
	}
	fee := 0
	for id, n := range fish {
		fee += mountFee(id) * n
	}
	options, _ := dlg["options"].([]string)
	at := len(options)
	if at > 0 && closesDialogue(npcData, options[at-1]) {
		at--
	}
	dlg["options"] = append(append(append([]string{}, options[:at]...), mountChoice), options[at:]...)
	dlg["option_labels"] = mergeLabels(dlg["option_labels"], map[string]string{mountChoice: fmt.Sprintf("🐟 Mount my huge fish (%d gold)", fee)})
}

// handleMountChoice mounts every huge fish you carry that you can pay for; nil
// when the choice isn't the mounting one.
func handleMountChoice(sess *session.GameSession, npcID, choice string) *GameActionResponse {
	if choice != mountChoice {
		return nil
	}
	state := &sess.SaveData
	if npcID != trophyMounter || !npc.OnDuty(state, npcID, "open_shop") {
		return fishRefusal("There's no one here to mount it.")
	}
	var made []string
	short := false
	for _, id := range sortedKeys(hugeFish(state)) {
		f := netFish(id)
		for gameutil.CountItem(state, id) > 0 {
			fee := mountFee(id)
			if gameutil.GetGoldQuantity(state) < fee {
				short = true
				break
			}
			if !gameutil.ConsumeItem(state, id, 1) {
				break
			}
			gameutil.DeductGold(state, fee)
			trophy := types.TrophyItemID(f.Species)
			if added, _ := inventory.AddItemToInventory(state, trophy, 1); added == 0 {
				sess.Ground.Add(world.GroundKey(state), trophy, 1)
			}
			made = append(made, fmt.Sprintf("%s (%d gold)", itemDisplayName(trophy), fee))
		}
	}
	if len(made) == 0 {
		return fishRefusal("You can't afford Finn's fee.")
	}
	msg := "Finn whistles, takes your catch into the back, and returns with it varnished on a plaque: " + strings.Join(made, ", ") + "."
	if short {
		msg += " You couldn't pay for the rest."
	}
	return &GameActionResponse{Success: true, Color: "green", Message: msg,
		Delta: map[string]interface{}{"npc_dialogue": map[string]interface{}{"action": "close"}}}
}
