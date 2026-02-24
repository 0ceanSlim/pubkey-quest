package types

// MonsterStats represents the six base ability scores for a monster
type MonsterStats struct {
	Strength     int `json:"strength"`
	Dexterity    int `json:"dexterity"`
	Constitution int `json:"constitution"`
	Intelligence int `json:"intelligence"`
	Wisdom       int `json:"wisdom"`
	Charisma     int `json:"charisma"`
}

// MonsterSpeed represents movement speeds
type MonsterSpeed struct {
	Walk  int `json:"walk"`
	Fly   int `json:"fly"`
	Swim  int `json:"swim"`
	Climb int `json:"climb"`
}

// MonsterSenses represents sensory abilities
type MonsterSenses struct {
	Darkvision       int `json:"darkvision"`
	PassivePerception int `json:"passive_perception"`
}

// MonsterHit represents the damage roll for a monster attack
type MonsterHit struct {
	Dice string `json:"dice"`
	Mod  int    `json:"mod"`
	Type string `json:"type"`
}

// MonsterAction represents an action a monster can take in combat
type MonsterAction struct {
	Name        string     `json:"name"`
	Type        string     `json:"type"` // "melee_attack", "ranged_attack"
	AttackBonus int        `json:"attack_bonus"`
	Reach       *int       `json:"reach"`
	Range       *int       `json:"range"`
	RangeLong   *int       `json:"range_long"`
	Hit         MonsterHit `json:"hit"`
}

// MonsterSpecialAbility represents a passive or active special trait
type MonsterSpecialAbility struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // "passive", "active"
}

// MonsterBehavior controls AI decision making
type MonsterBehavior struct {
	Aggression     string  `json:"aggression"`      // "aggressive", "defensive", "cautious", "berserker"
	FleeThreshold  float64 `json:"flee_threshold"`  // HP fraction to flee at (0.25 = 25%)
	PreferredRange int     `json:"preferred_range"` // 0-6
	TargetPriority string  `json:"target_priority"` // "lowest_hp", "highest_threat", "random"
	Relentless     bool    `json:"relentless"`
}

// LootGuaranteed is a drop that always appears after combat
type LootGuaranteed struct {
	Item     string `json:"item"`
	Quantity [2]int `json:"quantity"` // [min, max]
}

// LootEntry is one option within a loot tier
type LootEntry struct {
	Item     string  `json:"item"`
	Weight   int     `json:"weight"`
	Quantity *[2]int `json:"quantity,omitempty"`
}

// LootTier is a named tier with weighted entries
type LootTier struct {
	Name    string      `json:"name"`
	Weight  int         `json:"weight"`
	Entries []LootEntry `json:"entries"`
}

// LootTable defines the full drop table for a monster
type LootTable struct {
	Guaranteed []LootGuaranteed `json:"guaranteed"`
	Rolls      int              `json:"rolls"`
	Tiers      []LootTier       `json:"tiers"`
}

// MonsterData is the full monster stat block loaded from the database
type MonsterData struct {
	ID                    string                  `json:"id"`
	Name                  string                  `json:"name"`
	ChallengeRating       float64                 `json:"challenge_rating"`
	XP                    int                     `json:"xp"`
	Type                  string                  `json:"type"`
	Size                  string                  `json:"size"`
	ArmorClass            int                     `json:"armor_class"`
	HitPoints             int                     `json:"hit_points"`
	HPDice                string                  `json:"hp_dice"`
	Alignment             string                  `json:"alignment"`
	Tags                  []string                `json:"tags"`
	Img                   string                  `json:"img"`
	Environment           []string                `json:"environment"`
	Speed                 MonsterSpeed            `json:"speed"`
	Stats                 MonsterStats            `json:"stats"`
	SavingThrows          map[string]int          `json:"saving_throws"`
	Skills                map[string]int          `json:"skills"`
	DamageResistances     []string                `json:"damage_resistances"`
	DamageImmunities      []string                `json:"damage_immunities"`
	DamageVulnerabilities []string                `json:"damage_vulnerabilities"`
	ConditionImmunities   []string                `json:"condition_immunities"`
	Senses                MonsterSenses           `json:"senses"`
	PreferredRange        int                     `json:"preferred_range"`
	Actions               []MonsterAction         `json:"actions"`
	SpecialAbilities      []MonsterSpecialAbility `json:"special_abilities"`
	BonusActions          []interface{}           `json:"bonus_actions"`
	Reactions             []interface{}           `json:"reactions"`
	LegendaryActions      []interface{}           `json:"legendary_actions"`
	LootTable             LootTable               `json:"loot_table"`
	Behavior              MonsterBehavior         `json:"behavior"`
}

// AdvancementEntry is one row of the XP progression table
type AdvancementEntry struct {
	ExperiencePoints int     `json:"ExperiencePoints"`
	Level            int     `json:"Level"`
	XPMultiplier     float64 `json:"XPMultiplier"`
}

// LootDrop is a single item drop result after combat
type LootDrop struct {
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
}

// CombatCondition is a condition applied to a combatant
type CombatCondition struct {
	Name           string `json:"name"`
	DurationRounds int    `json:"duration_rounds"` // -1 = permanent until removed
	SaveDC         int    `json:"save_dc,omitempty"`
	SaveStat       string `json:"save_stat,omitempty"`
}

// PlayerCombatState tracks per-turn resource usage and status for the player
type PlayerCombatState struct {
	CurrentHP          int               `json:"current_hp"`
	MaxHP              int               `json:"max_hp"`
	ActionUsed         bool              `json:"action_used"`
	BonusActionUsed    bool              `json:"bonus_action_used"`
	MovementUsed       bool              `json:"movement_used"`
	HeldPosition       bool              `json:"held_position"` // True when player held still this turn (readied attack)
	Dodging            bool              `json:"dodging"`       // True until start of next turn — monster attacks at disadvantage
	DeathSaveSuccesses int               `json:"death_save_successes"`
	DeathSaveFailures  int               `json:"death_save_failures"`
	IsUnconscious      bool              `json:"is_unconscious"`
	IsStable           bool              `json:"is_stable"`
	Conditions         []CombatCondition `json:"conditions"`
}

// MonsterInstance is a live monster in the current combat encounter
type MonsterInstance struct {
	TemplateID string           `json:"template_id"` // ID from monster JSON
	InstanceID string           `json:"instance_id"` // Unique per-combat ID (for future multi-monster)
	Name       string           `json:"name"`
	CurrentHP  int              `json:"current_hp"`
	MaxHP      int              `json:"max_hp"`
	ArmorClass int              `json:"armor_class"`
	Initiative int              `json:"initiative"`
	Conditions []CombatCondition `json:"conditions"`
	IsAlive    bool             `json:"is_alive"`
	Data       MonsterData      `json:"data"` // Full stat block
}

// PartyCombatant represents the player (or future companion) in combat
type PartyCombatant struct {
	Type               string            `json:"type"` // "player", future: "companion"
	ID                 string            `json:"id"`   // npub
	IsPlayerControlled bool              `json:"is_player_controlled"`
	CombatState        PlayerCombatState `json:"combat_state"`
}

// InitiativeEntry records a combatant's position in the initiative order
type InitiativeEntry struct {
	ID         string `json:"id"`         // npub for player, instance_id for monster
	Type       string `json:"type"`       // "player" or "monster"
	Initiative int    `json:"initiative"`
	DEXScore   int    `json:"dex_score"` // For tie-breaking
}

// CombatSession holds all in-memory state for an active combat encounter.
// This is NEVER written to the save file — it lives only in GameSession memory.
type CombatSession struct {
	Party             []PartyCombatant  `json:"party"`
	Monsters          []MonsterInstance `json:"monsters"`
	Initiative        []InitiativeEntry `json:"initiative"`
	CurrentTurnIndex  int               `json:"current_turn_index"`
	Round             int               `json:"round"`
	Range             int               `json:"range"`
	Log               []string          `json:"log"`
	EnvironmentID     string            `json:"environment_id"`
	IsSurprised       bool              `json:"is_surprised"`   // Player was surprised (monster acts first)
	Phase             string            `json:"phase"`          // "active", "loot", "victory", "defeat", "death_saves"
	TurnPhase         string            `json:"turn_phase"`     // "move" or "action" — player's sub-turn step
	LootRolled        []LootDrop        `json:"loot_rolled,omitempty"`
	LevelUpPending     bool              `json:"level_up_pending"`
	XPEarnedThisFight  int               `json:"xp_earned_this_fight"`
	AmmoUsedThisCombat int               `json:"ammo_used_this_combat"`
}
