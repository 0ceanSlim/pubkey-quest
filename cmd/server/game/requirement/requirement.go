// Package requirement evaluates POIRequirement gates shared by quests
// (availability), POI/encounter nodes (choice + whole-content gating), and
// dialogue. It is deliberately pure policy: the derived player facts a
// requirement is checked against (skills, level, quest points, inventory,
// identity) are supplied through a Context, so the canonical derivations live
// once in their own packages and this code stays trivially testable.
package requirement

import (
	"strings"

	"pubkey-quest/types"
)

// Context supplies the derived player facts a requirement is checked against.
// Implementations compose the canonical derivations (skill formulas, level
// from XP, quest points from the completed-quest list, inventory lookup) — see
// the production builder that the quest engine wires up.
type Context interface {
	SkillValue(skillID string) int // derived skill score (e.g. athletics)
	StatValue(statID string) int   // effective ability score (base + allocations + effects)
	Level() int                    // derived from experience
	QuestPoints() int              // derived from the completed-quest list
	HasItem(itemID string) bool    // present anywhere the player can draw from
	Class() string
	Race() string
	Alignment() string
	IsQuestCompleted(questID string) bool
}

// Result reports whether every requirement passed (AND semantics) and, when it
// did not, exactly which ones failed — so callers can render a player-facing
// "locked because…" using each requirement's Description.
type Result struct {
	OK      bool
	Missing []types.POIRequirement
}

// Evaluate checks a requirement set with AND semantics: OK is true only when
// every requirement is satisfied. An empty/nil set passes. For OR semantics,
// content authors duplicate the choice (per the design doc).
func Evaluate(reqs []types.POIRequirement, ctx Context) Result {
	res := Result{OK: true}
	for _, req := range reqs {
		if !EvaluateOne(req, ctx) {
			res.OK = false
			res.Missing = append(res.Missing, req)
		}
	}
	return res
}

// EvaluateOne reports whether a single requirement is satisfied. Unknown
// requirement types fail closed: content that names a gate the engine does not
// understand should block, not silently open.
func EvaluateOne(req types.POIRequirement, ctx Context) bool {
	switch req.Type {
	case "skill":
		return ctx.SkillValue(req.ID) >= req.Min
	case "stat":
		return ctx.StatValue(req.ID) >= req.Min
	case "level":
		return ctx.Level() >= req.Min
	case "quest_points":
		return ctx.QuestPoints() >= req.Min
	case "item":
		return ctx.HasItem(req.ID)
	case "class":
		return containsFold(req.Values, ctx.Class())
	case "race":
		return containsFold(req.Values, ctx.Race())
	case "alignment":
		return containsFold(req.Values, ctx.Alignment())
	case "quest_completed":
		return ctx.IsQuestCompleted(req.ID)
	default:
		return false
	}
}

// containsFold reports whether want matches any of values, case-insensitively.
func containsFold(values []string, want string) bool {
	for _, v := range values {
		if strings.EqualFold(v, want) {
			return true
		}
	}
	return false
}
