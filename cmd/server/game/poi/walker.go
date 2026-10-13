// Package poi is the POI / encounter node-walker — the engine that runs a
// discovered point of interest (or an authored encounter) as a graph of nodes.
// Both POIs and encounters share the same node schema (types.POIStep), so this
// one walker drives both.
//
// The walker resolves a single node at a time: it applies that node's effects to
// the save, then reports what to present to the player (text + outcome) and
// where the walk goes next. The server holds the current node in a session and
// calls Resolve again on each player advance. Side-effecting collaborators are
// injected (Deps) so the resolution logic stays pure and unit-testable.
package poi

import (
	"fmt"
	"math/rand"
	"strings"

	"pubkey-quest/cmd/server/game/locks"

	"pubkey-quest/cmd/server/game/requirement"
	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/types"
)

// Choice is a player-selectable option presented on a choice node (already
// filtered to those whose requirements are met).
type Choice struct {
	Label string `json:"label"`
	Next  string `json:"next"`
}

// StepResult is the outcome of resolving one node — everything the UI needs to
// present it and everything the session needs to advance.
type StepResult struct {
	NodeID   string   `json:"node_id"`
	Text     string   `json:"text,omitempty"`
	Outcome  []string `json:"outcome,omitempty"` // what happened: check result, loot, damage…
	Choices  []Choice `json:"choices,omitempty"` // present on choice nodes
	Combat   string   `json:"combat,omitempty"`  // monster id — the caller bridges into combat
	// CombatCount / CombatSurprise carry a monster node's authored group size and
	// ambush flag through to the combat bridge.
	CombatCount    int      `json:"-"`
	CombatSurprise bool     `json:"-"`
	CombatArena    []string `json:"-"`
	Next     string   `json:"next,omitempty"`    // node to resolve on "continue" (empty if choices/terminal)
	Terminal bool     `json:"terminal"`

	// CheckSkill / CheckSuccess report the result of a check (or passive_check)
	// node so the caller can record a SkillCheckPassed event (advancing quest
	// "check" objectives) without the walker importing the event recorder —
	// keeping resolution pure. Empty/false on every other node type.
	CheckSkill   string `json:"-"`
	CheckSuccess bool   `json:"-"`

	// Waypoint is the named spot this node reached (quest explore objectives).
	Waypoint string `json:"-"`

	// NPCs are the people an npc_interaction node hands the player to; the
	// walk ends and the UI offers to talk to them where they stand.
	NPCs []string `json:"npcs,omitempty"`
}

// Deps injects the systems a node may touch, so the walker stays testable.
type Deps struct {
	Ctx         requirement.Context                    // choice gating + effective skill values
	Rng         *rand.Rand                             // check rolls
	GrantReward func(save *types.SaveFile, r *types.POIReward) // XP/gold/items/effect
	ApplyEffect func(save *types.SaveFile, effectID string)    // status effect
	AddItem     func(save *types.SaveFile, itemID string, qty int)
	// PayCost takes a transaction node's price, all or nothing; false when the
	// player can't afford it.
	PayCost func(save *types.SaveFile, c *types.POICost) bool
	// Locks serves lock nodes (nil: lock nodes only take their key).
	Locks LockDeps
	// ChangeFatigue raises (or lowers) the fatigue stat — an effect node whose
	// id is FatigueEffect.
	ChangeFatigue func(save *types.SaveFile, amount int)
}

// FatigueEffect is the effect id that means the fatigue survival stat rather
// than a status effect: {"type":"effect","effect":{"id":"fatigue","amount":2}}.
const FatigueEffect = "fatigue"

// LockDeps is what a lock node needs from the game: today's state of the lock,
// the player's lockpicks, a quick pick attempt (which uses a pick on failure,
// passes time and remembers the result), and item names for the key button.
type LockDeps interface {
	State(node string) (open, jammed bool)
	Picks() int
	Quick(node string, lock types.Lock) locks.Result
	ItemName(id string) string
}

// Lock node sub-steps: a lock node's choices lead to "<node>#key" etc., which
// resolve against the same node. "#pick" is the minigame — the client opens it
// rather than advancing — so on the server it just re-presents the lock.
const (
	LockKey   = "#key"
	LockQuick = "#quick"
	LockPick  = "#pick"
	LockLeave = "#leave"
)

// BaseNode strips a lock sub-step from a node id ("door#quick" → "door").
func BaseNode(nodeID string) string {
	base, _, _ := strings.Cut(nodeID, "#")
	return base
}

// Resolve applies a node's effects to the save and computes its presentation.
// nodeID is the node's key (the map key in POIData.Nodes).
func Resolve(node types.POIStep, nodeID string, save *types.SaveFile, deps Deps) StepResult {
	res := StepResult{NodeID: nodeID, Text: node.Text, Waypoint: node.Waypoint}

	// A gated node the player doesn't qualify for does nothing and turns them
	// away — to its failure branch, or out of the POI.
	if len(node.Requirements) > 0 && !requirement.Evaluate(node.Requirements, deps.Ctx).OK {
		res.Waypoint = ""
		res.Outcome = append(res.Outcome, nonEmpty(node.FailureText, "You can't go that way."))
		res.Next = node.FailureNext
		res.Terminal = node.FailureNext == ""
		return res
	}

	switch node.Type {
	case types.POIStepNarrative:
		res.Next = node.Next

	case types.POIStepChoice:
		for _, ch := range node.Choices {
			if requirement.Evaluate(ch.Requirements, deps.Ctx).OK {
				res.Choices = append(res.Choices, Choice{Label: ch.Label, Next: ch.Next})
			}
		}

	case types.POIStepCheck, types.POIStepPassiveCheck:
		r := skillcheck.Resolve(deps.Ctx.SkillValue(node.Skill), node.DC, deps.Rng)
		res.CheckSkill = node.Skill
		res.CheckSuccess = r.Success
		if r.Success {
			res.Outcome = append(res.Outcome, nonEmpty(node.SuccessText, fmt.Sprintf("Success (%s check).", node.Skill)))
			res.Next = node.SuccessNext
		} else {
			res.Outcome = append(res.Outcome, nonEmpty(node.FailureText, fmt.Sprintf("Failure (%s check).", node.Skill)))
			res.Next = node.FailureNext
		}

	case types.POIStepReward:
		if node.Reward != nil && deps.GrantReward != nil {
			deps.GrantReward(save, node.Reward)
			res.Outcome = append(res.Outcome, rewardSummary(node.Reward))
		}
		res.Next = node.Next

	case types.POIStepLoot:
		res.Outcome = append(res.Outcome, grantLoot(node.LootTable, save, deps)...)
		res.Next = node.Next

	case types.POIStepDamage:
		if node.Damage != nil && node.Damage.Amount > 0 {
			save.HP -= node.Damage.Amount
			if save.HP < 0 {
				save.HP = 0
			}
			res.Outcome = append(res.Outcome, fmt.Sprintf("You take %d %s damage.", node.Damage.Amount, node.Damage.Type))
		}
		res.Next = node.Next

	case types.POIStepEffect:
		// "fatigue" isn't a status effect but the survival stat: it rises by
		// Amount (and the Tired/Fatigued statuses follow from it).
		if node.Effect != nil && node.Effect.ID == FatigueEffect {
			if deps.ChangeFatigue != nil && node.Effect.Amount != 0 {
				deps.ChangeFatigue(save, node.Effect.Amount)
				res.Outcome = append(res.Outcome, fmt.Sprintf("Fatigue %+d.", node.Effect.Amount))
			}
			res.Next = node.Next
			break
		}
		if node.Effect != nil && node.Effect.ID != "" && deps.ApplyEffect != nil {
			deps.ApplyEffect(save, node.Effect.ID)
			res.Outcome = append(res.Outcome, fmt.Sprintf("You gain %s.", node.Effect.ID))
		}
		res.Next = node.Next

	case types.POIStepTransaction:
		if node.Cost != nil && deps.PayCost != nil && !deps.PayCost(save, node.Cost) {
			res.Outcome = append(res.Outcome, nonEmpty(node.FailureText, "You can't afford that."))
			res.Next = node.FailureNext
			res.Terminal = node.FailureNext == ""
			return res
		}
		if node.Reward != nil && deps.GrantReward != nil {
			deps.GrantReward(save, node.Reward)
			res.Outcome = append(res.Outcome, rewardSummary(node.Reward))
		}
		res.Next = node.Next

	case types.POIStepMonster:
		// The caller starts combat with this monster and resumes the POI at Next
		// once it's won. (Bridge wired with the session.)
		res.Combat = node.MonsterID
		res.CombatCount = node.Count
		res.CombatSurprise = node.Surprise
		res.CombatArena = node.Arena
		res.Next = node.Next

	case types.POIStepNPCInteraction:
		res.NPCs = node.NPCIDs
		res.Terminal = true

	case types.POIStepExit:
		res.Terminal = true

	case types.POIStepLock:
		resolveLock(node, nodeID, save, deps, &res)
	}

	if node.IsTerminal {
		res.Terminal = true
	}
	return res
}

// grantLoot grants a loot table: every guaranteed entry always drops, then the
// table rolls Rolls times against its weighted Tiers (one weighted entry per
// roll). This is what makes gathering yields varied — a mining node drops ore
// every time (guaranteed) but only sometimes turns up a gem (a rare tier).
// Quantities may be a fixed number or a [min,max] range.
func grantLoot(table *types.POILootTable, save *types.SaveFile, deps Deps) []string {
	if table == nil || deps.AddItem == nil {
		return nil
	}
	var out []string
	for _, entry := range table.Guaranteed {
		out = appendGrant(out, entry, save, deps)
	}

	if len(table.Tiers) > 0 {
		rolls := table.Rolls
		if rolls <= 0 {
			rolls = 1
		}
		for i := 0; i < rolls; i++ {
			if tier := pickTier(table.Tiers, deps.Rng); tier != nil {
				if entry := pickEntry(tier.Entries, deps.Rng); entry != nil {
					out = appendGrant(out, *entry, save, deps)
				}
			}
		}
	}
	return out
}

// appendGrant grants one loot entry (resolving a range quantity) and appends its
// outcome line. A qty of 0 (empty tier miss) grants nothing.
func appendGrant(out []string, entry types.POILootEntry, save *types.SaveFile, deps Deps) []string {
	qty := resolveQuantity(entry.Quantity, deps.Rng)
	if entry.Item == "" || qty <= 0 {
		return out
	}
	deps.AddItem(save, entry.Item, qty)
	return append(out, fmt.Sprintf("You find %s ×%d.", entry.Item, qty))
}

// pickTier chooses one tier weighted by Tier.Weight. A tier with no entries is
// still a valid outcome (a "nothing" bucket) — give it entries or leave it empty
// to model "sometimes you find nothing".
func pickTier(tiers []types.POILootTier, rng *rand.Rand) *types.POILootTier {
	total := 0
	for _, t := range tiers {
		if t.Weight > 0 {
			total += t.Weight
		}
	}
	if total <= 0 {
		return nil
	}
	roll := rngIntn(rng, total)
	for i := range tiers {
		if tiers[i].Weight <= 0 {
			continue
		}
		roll -= tiers[i].Weight
		if roll < 0 {
			return &tiers[i]
		}
	}
	return nil
}

// pickEntry chooses one entry within a tier weighted by Entry.Weight (an entry
// with no weight counts as weight 1). Returns nil for an empty tier.
func pickEntry(entries []types.POILootEntry, rng *rand.Rand) *types.POILootEntry {
	if len(entries) == 0 {
		return nil
	}
	total := 0
	for _, e := range entries {
		total += entryWeight(e)
	}
	roll := rngIntn(rng, total)
	for i := range entries {
		roll -= entryWeight(entries[i])
		if roll < 0 {
			return &entries[i]
		}
	}
	return &entries[len(entries)-1]
}

func entryWeight(e types.POILootEntry) int {
	if e.Weight > 0 {
		return e.Weight
	}
	return 1
}

// rngIntn is a nil-safe rng.Intn (falls back to 0 so loot is deterministic in
// tests that don't supply an rng).
func rngIntn(rng *rand.Rand, n int) int {
	if n <= 0 {
		return 0
	}
	if rng == nil {
		return 0
	}
	return rng.Intn(n)
}

// resolveQuantity reads a loot quantity: a plain number, or a [min,max] 2-element
// array rolled inclusively.
func resolveQuantity(q any, rng *rand.Rand) int {
	switch v := q.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case []any:
		if len(v) == 2 {
			lo, hi := quantityToInt(v[0]), quantityToInt(v[1])
			if hi < lo {
				lo, hi = hi, lo
			}
			if hi == lo {
				return lo
			}
			return lo + rngIntn(rng, hi-lo+1)
		}
		if len(v) == 1 {
			return quantityToInt(v[0])
		}
	}
	return 0
}

func quantityToInt(q any) int {
	switch n := q.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

func rewardSummary(r *types.POIReward) string {
	parts := []string{}
	if r.XP > 0 {
		parts = append(parts, fmt.Sprintf("%d XP", r.XP))
	}
	if r.Gold > 0 {
		parts = append(parts, fmt.Sprintf("%d gold", r.Gold))
	}
	for _, it := range r.Items {
		q := it.Quantity
		if q <= 0 {
			q = 1
		}
		parts = append(parts, fmt.Sprintf("%s ×%d", it.ID, q))
	}
	if len(parts) == 0 {
		return ""
	}
	out := "You gain "
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out + "."
}

func nonEmpty(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

// resolveLock plays a lock node: an unlocked (picked today) lock lets you
// through; "#key" opens it with its key; "#quick" makes one quick pick attempt;
// "#leave" walks away; anything else presents the ways to get past it.
func resolveLock(node types.POIStep, nodeID string, save *types.SaveFile, deps Deps, res *StepResult) {
	base := BaseNode(nodeID)
	action := strings.TrimPrefix(nodeID, base)
	if node.Lock == nil {
		res.Next = node.SuccessNext
		return
	}
	lock := *node.Lock
	open, jammed, picks := false, false, 0
	if deps.Locks != nil {
		open, jammed = deps.Locks.State(base)
		picks = deps.Locks.Picks()
	}
	hasKey := lock.Key != "" && deps.Ctx != nil && deps.Ctx.HasItem(lock.Key)

	opened := func(line string) {
		res.Outcome = append(res.Outcome, line)
		res.Next = node.SuccessNext
		res.Terminal = node.SuccessNext == ""
	}
	switch {
	case open:
		opened("It's unlocked.")
		return
	case action == LockKey && hasKey:
		opened(fmt.Sprintf("You unlock it with the %s.", itemName(deps, lock.Key)))
		return
	case action == LockLeave:
		res.Next = node.FailureNext
		res.Terminal = node.FailureNext == ""
		return
	case action == LockQuick && deps.Locks != nil:
		if why := locks.CanPick(lock, jammed, picks); why != "" {
			res.Outcome = append(res.Outcome, why)
			break
		}
		r := deps.Locks.Quick(base, lock)
		if r.Opened {
			opened(r.Describe())
			return
		}
		res.Outcome = append(res.Outcome, r.Describe())
		open, jammed = deps.Locks.State(base)
		picks = deps.Locks.Picks()
	}

	// The ways past it.
	if hasKey {
		res.Choices = append(res.Choices, Choice{Label: "Use the " + itemName(deps, lock.Key), Next: base + LockKey})
	}
	if lock.DC > 0 && deps.Locks != nil {
		switch why := locks.CanPick(lock, jammed, picks); {
		case why != "":
			if action == "" || action == LockQuick {
				res.Outcome = appendOnce(res.Outcome, why)
			}
		default:
			res.Choices = append(res.Choices,
				Choice{Label: fmt.Sprintf("Pick the lock (%d lockpicks)", picks), Next: base + LockPick},
				Choice{Label: "Quick attempt", Next: base + LockQuick})
		}
	} else if lock.DC <= 0 && !hasKey && action == "" {
		res.Outcome = append(res.Outcome, "There's no picking this one. You'd need its key.")
	}
	leave := node.FailureNext
	if leave == "" {
		leave = base + LockLeave
	}
	res.Choices = append(res.Choices, Choice{Label: "Leave it", Next: leave})
}

func itemName(deps Deps, id string) string {
	if deps.Locks != nil {
		if n := deps.Locks.ItemName(id); n != "" {
			return n
		}
	}
	return strings.ReplaceAll(id, "-", " ")
}

func appendOnce(lines []string, line string) []string {
	for _, l := range lines {
		if l == line {
			return lines
		}
	}
	return append(lines, line)
}
