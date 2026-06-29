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
	Next     string   `json:"next,omitempty"`    // node to resolve on "continue" (empty if choices/terminal)
	Terminal bool     `json:"terminal"`

	// CheckSkill / CheckSuccess report the result of a check (or passive_check)
	// node so the caller can record a SkillCheckPassed event (advancing quest
	// "check" objectives) without the walker importing the event recorder —
	// keeping resolution pure. Empty/false on every other node type.
	CheckSkill   string `json:"-"`
	CheckSuccess bool   `json:"-"`
}

// Deps injects the systems a node may touch, so the walker stays testable.
type Deps struct {
	Ctx         requirement.Context                    // choice gating + effective skill values
	Rng         *rand.Rand                             // check rolls
	GrantReward func(save *types.SaveFile, r *types.POIReward) // XP/gold/items/effect
	ApplyEffect func(save *types.SaveFile, effectID string)    // status effect
	AddItem     func(save *types.SaveFile, itemID string, qty int)
}

// Resolve applies a node's effects to the save and computes its presentation.
// nodeID is the node's key (the map key in POIData.Nodes).
func Resolve(node types.POIStep, nodeID string, save *types.SaveFile, deps Deps) StepResult {
	res := StepResult{NodeID: nodeID, Text: node.Text}

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
		res.Outcome = append(res.Outcome, grantGuaranteedLoot(node.LootTable, save, deps)...)
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
		if node.Effect != nil && node.Effect.ID != "" && deps.ApplyEffect != nil {
			deps.ApplyEffect(save, node.Effect.ID)
			res.Outcome = append(res.Outcome, fmt.Sprintf("You gain %s.", node.Effect.ID))
		}
		res.Next = node.Next

	case types.POIStepTransaction:
		if node.Reward != nil && deps.GrantReward != nil {
			deps.GrantReward(save, node.Reward)
			res.Outcome = append(res.Outcome, rewardSummary(node.Reward))
		}
		res.Next = node.Next

	case types.POIStepMonster:
		// The caller starts combat with this monster and resumes the POI at Next
		// once it's won. (Bridge wired with the session.)
		res.Combat = node.MonsterID
		res.Next = node.Next

	case types.POIStepExit, types.POIStepNPCInteraction:
		res.Terminal = true
	}

	if node.IsTerminal {
		res.Terminal = true
	}
	return res
}

// grantGuaranteedLoot grants a loot table's guaranteed entries (integer
// quantity). Weighted-tier rolls are a follow-up — guaranteed covers the common
// "you always find X here" case.
func grantGuaranteedLoot(table *types.POILootTable, save *types.SaveFile, deps Deps) []string {
	if table == nil || deps.AddItem == nil {
		return nil
	}
	var out []string
	for _, entry := range table.Guaranteed {
		qty := quantityToInt(entry.Quantity)
		if qty <= 0 {
			qty = 1
		}
		deps.AddItem(save, entry.Item, qty)
		out = append(out, fmt.Sprintf("You find %s ×%d.", entry.Item, qty))
	}
	return out
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
