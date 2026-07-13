package types

// FeatStatGrant is a feat's ability-score bonus. A single-element Choices list is
// a fixed grant (e.g. Actor → Charisma); multiple choices are a half-feat where the
// player picks one (e.g. Athlete → Strength or Dexterity).
type FeatStatGrant struct {
	Amount  int      `json:"amount"`
	Choices []string `json:"choices"`
}

// FeatChoice is a non-stat choice a feat requires at selection (e.g. Elemental
// Adept picks a damage type). The picked option is stored suffixed on the feat id
// ("elemental-adept:fire") and read by the effect hook. Kind is descriptive.
type FeatChoice struct {
	Kind    string   `json:"kind"`
	Options []string `json:"options"`
}

// Feat is a selectable feat (docs/draft/feats-progression.md). At a class's
// feat-eligible levels a character may take a feat instead of that level's ability
// point. The alpha MVP set is stat/HP grants; combat-hook feats (War Caster, etc.)
// come later.
type Feat struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	// Prerequisite: "" = none. "spellcaster" = must be able to cast (reserved for
	// the casting feats). Evaluated at selection time.
	Prerequisite string         `json:"prerequisite,omitempty"`
	StatGrant    *FeatStatGrant `json:"stat_grant,omitempty"`
	Choice       *FeatChoice    `json:"choice,omitempty"`       // a non-stat pick (e.g. damage type)
	HPPerLevel   int            `json:"hp_per_level,omitempty"` // Tough: +N max HP per level
	Effects      []string       `json:"effects,omitempty"`      // display-only lines
}
