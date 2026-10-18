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
	Dice    string             `json:"dice"`
	Mod     int                `json:"mod"`
	Type    string             `json:"type"`
	Special *MonsterHitSpecial `json:"special,omitempty"`
}

// MonsterHitSpecial is an on-hit rider authored in the monster data: a save the
// player must make or suffer a condition (§15/§23). `type` is "save" (Effect
// names the condition, e.g. "knocked_prone"/"restrained"/"paralyzed") or
// "poison_save" (poisoned). Non-condition specials (pull, life_steal, …) are
// ignored by the conditions engine.
type MonsterHitSpecial struct {
	Type    string `json:"type"`
	Ability string `json:"ability,omitempty"`
	DC      int    `json:"dc,omitempty"`
	Effect  string `json:"effect,omitempty"`
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
	KillBonusXP           int                     `json:"kill_bonus_xp,omitempty"` // Flat bonus XP for the kill itself (tougher monsters / set by a POI step); 0 = none
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
	AbilityPoints    int     `json:"AbilityPoints"` // Ability points granted at this level (spend on stats; feats consume one — see docs/draft/feats-progression.md)
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
	// Set when a spell imposed it: the spell and the member who cast it. A
	// concentration spell's conditions end with the concentration; a compelled
	// duel's target is held to its caster.
	Source   string `json:"source,omitempty"`
	CasterID string `json:"caster_id,omitempty"`
	// Damage dealt to the afflicted creature at the start of each of its turns
	// (searing smite's flames, acid arrow's burn). "" for none.
	DamageDice string `json:"damage_dice,omitempty"`
	DamageType string `json:"damage_type,omitempty"`
}

// SpellArea is a save spell's area of effect, authored on the spell JSON as
// `aoe`. Every enemy caught in it makes its own save. Radius is in grid cells
// (5 ft each): a sphere's or self_radius's radius, a cone's or line's length.
type SpellArea struct {
	// Shape: "sphere" (centred on the target's cell), "self_radius" (around
	// the caster, who is spared), "cone" or "line" (from the caster toward
	// the target).
	Shape  string `json:"shape"`
	Radius int    `json:"radius"`
}

// SpellCondition is a spell's condition rider, authored on the spell JSON as
// `applies_condition` (lands on the target of a save or attack spell) or as a
// combat buff's `hit_condition` (lands on whoever the next weapon hit strikes).
type SpellCondition struct {
	Condition string `json:"condition"`
	// Rounds the condition lasts; 0 = the rest of the fight.
	Rounds int `json:"rounds,omitempty"`
	// SaveEachTurn names the stat the creature saves with at the end of each
	// of its turns to shake it off (entangle: strength). "" = no re-save.
	SaveEachTurn string `json:"save_each_turn,omitempty"`
	// Save is the stat a hit_condition is resisted with when the hit lands
	// (a smite's rider). Ignored on applies_condition: there the spell's own
	// save_type already decided it.
	Save string `json:"save,omitempty"`
	// CreatureTypes limits the rider to these monster types ("beast",
	// "humanoid"); others shrug it off. Empty = any creature.
	CreatureTypes []string `json:"creature_types,omitempty"`
	DamageDice    string   `json:"damage_dice,omitempty"`
	DamageType    string   `json:"damage_type,omitempty"`
}

// SpellBuff is a spell's effect on one combatant for the rest of a fight
// (bless, shield, a smite waiting on its hit, hex on a foe). It is authored on
// the spell JSON as `combat_buff` with these same keys and copied onto the
// target's combat state when the spell is cast. Memory-only, like all combat.
type SpellBuff struct {
	SpellID  string `json:"spell_id,omitempty"`
	Name     string `json:"name,omitempty"`
	CasterID string `json:"caster_id,omitempty"`
	// Target (authored): "self" (default), "ally" (self or the chosen ally),
	// "party" (the caster and up to two allies) or "enemy" (the buff stays on
	// the caster and marks the chosen foe — hex, hunter's mark).
	Target string `json:"target,omitempty"`
	// Rounds counts down at the start of the holder's turn; 0 = the rest of the fight.
	Rounds int `json:"rounds,omitempty"`

	ACBonus     int    `json:"ac_bonus,omitempty"`
	ACFloor     int    `json:"ac_floor,omitempty"`     // AC can't be lower than this (barkskin)
	AttackBonus int    `json:"attack_bonus,omitempty"` // flat, to weapon attack rolls (magic weapon)
	AttackDice  string `json:"attack_dice,omitempty"`  // rolled and added to attack rolls (bless)
	SaveDice    string `json:"save_dice,omitempty"`    // rolled and added to saving throws (bless, resistance)

	// Extra damage when the holder hits. HitOnce spends the buff on the first
	// hit (smites); MeleeOnly limits it to melee weapon hits; HitSpells lets it
	// ride spell-attack hits too (hex). MarkedID limits it to one foe.
	HitDice      string          `json:"hit_dice,omitempty"`
	HitType      string          `json:"hit_type,omitempty"`
	HitBonus     int             `json:"hit_bonus,omitempty"`
	HitOnce      bool            `json:"hit_once,omitempty"`
	// Hits spends the hit damage after this many hits (a weapon coating's
	// "next 3 hits"); 0 = every hit while the buff lasts.
	Hits int `json:"hits,omitempty"`
	MeleeOnly    bool            `json:"melee_only,omitempty"`
	HitSpells    bool            `json:"hit_spells,omitempty"`
	HitCondition *SpellCondition `json:"hit_condition,omitempty"`
	MarkedID     string          `json:"marked_id,omitempty"`

	// Defence.
	Resist                []string `json:"resist,omitempty"`                 // halve damage of these types
	AttackersDisadvantage bool     `json:"attackers_disadvantage,omitempty"` // blur, invisibility
	RangedDefense         bool     `json:"ranged_defense,omitempty"`         // ranged attacks at the holder have disadvantage (fog)
	WardTypes             []string `json:"ward_types,omitempty"`             // creatures of these types attack the holder at disadvantage
	Immune                []string `json:"immune,omitempty"`                 // conditions that can't take hold (heroism: frightened)
	Decoys                int      `json:"decoys,omitempty"`                 // illusory doubles: each takes one attack that would have hit
	Sanctuary             bool     `json:"sanctuary,omitempty"`              // an attacker must pass a WIS save or lose the attack
	EndsOnAttack          bool     `json:"ends_on_attack,omitempty"`         // drops when the holder attacks or casts at a foe
	DeathWard             bool     `json:"death_ward,omitempty"`             // the first drop to 0 HP leaves the holder at 1
	EasyEscape            bool     `json:"easy_escape,omitempty"`            // fleeing needs no distance and always works

	// Temporary hit points: TempHP is rolled when the spell lands;
	// TempHPEachTurn grants the caster's spellcasting modifier (min 1) at the
	// start of each of the holder's turns (heroism).
	TempHP         string `json:"temp_hp,omitempty"`
	TempHPEachTurn int    `json:"temp_hp_each_turn,omitempty"`

	// Retaliate deals damage back to a melee attacker that hits the holder;
	// RetaliateWhileTempHP limits it to while temporary hit points remain
	// (armor of agathys).
	Retaliate            string `json:"retaliate,omitempty"`
	RetaliateType        string `json:"retaliate_type,omitempty"`
	RetaliateWhileTempHP bool   `json:"retaliate_while_temp_hp,omitempty"`

	// Offence and movement.
	NextAttackAdvantage bool     `json:"next_attack_advantage,omitempty"` // spent on the next attack roll
	MeleeStat           string   `json:"melee_stat,omitempty"`            // "spellcasting": melee weapon attacks use the casting ability
	WeaponDice          string   `json:"weapon_dice,omitempty"`           // replaces the weapon's damage die
	Weapons             []string `json:"weapons,omitempty"`               // item ids MeleeStat/WeaponDice apply to (empty = any melee)
	ExtraMove           int      `json:"extra_move,omitempty"`            // extra grid cells of movement per turn
	Revive              bool     `json:"revive,omitempty"`                // cast on a downed ally: they get up at 1 HP

	SaveDC int `json:"save_dc,omitempty"` // the caster's spell save DC, filled in at cast (sanctuary)
}

// Position is an X,Y coordinate on the combat grid.
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ResourcePool is a martial class's combat-scoped resource (Rage/Stamina/Ki/
// Cunning) used to pay for class abilities (M5 §12). Memory-only — the max is
// re-derived from class + level each fight (mirrors game-data/systems/
// class-resources.json), and the current value never persists to the save.
// The regen rates are carried here so the combat loop needn't reload the config.
type ResourcePool struct {
	Type        string `json:"type"`  // rage / stamina / ki / cunning
	Label       string `json:"label"` // display name ("Rage")
	Current     int    `json:"current"`
	Max         int    `json:"max"`
	PerTurn     int    `json:"-"` // regained at the end of your turn
	PerHitTaken int    `json:"-"` // regained when you take a hit (barbarian rage builds this way)
	PerCrit     int    `json:"-"` // regained when you land a crit (rogue cunning)
}

// PlayerCombatState tracks per-turn resource usage and status for the player
type PlayerCombatState struct {
	CurrentHP          int               `json:"current_hp"`
	MaxHP              int               `json:"max_hp"`
	ActionUsed         bool              `json:"action_used"`
	BonusActionUsed    bool              `json:"bonus_action_used"`
	MovementBudget     int               `json:"movement_budget"` // Max cells per turn (Speed / 5)
	MovementSpent      int               `json:"movement_spent"`  // Cells used this turn
	HeldPosition       bool              `json:"held_position"`   // True when player didn't move this turn (readied attack)
	Dodging            bool              `json:"dodging"`         // True until start of next turn — monster attacks at disadvantage
	DeathSaveSuccesses int               `json:"death_save_successes"`
	DeathSaveFailures  int               `json:"death_save_failures"`
	IsUnconscious      bool              `json:"is_unconscious"`
	IsStable           bool              `json:"is_stable"`
	ReactionUsed       bool              `json:"reaction_used"` // Reaction consumed this round (OA)
	Disengaged         bool              `json:"disengaged"`    // Used Disengage action this turn — no OAs provoked
	Conditions         []CombatCondition `json:"conditions"`

	// Class-ability state (M5 §12) — all memory-only, initialised at combat start.
	Resource           *ResourcePool `json:"resource,omitempty"`             // martial resource pool; nil for casters
	RageTurnsLeft      int           `json:"rage_turns_left,omitempty"`      // barbarian rage: turns remaining (0 = not raging)
	RageDamageBonusPct int           `json:"rage_damage_bonus_pct,omitempty"` // % bonus to weapon damage while raging
	RageResistPct      int           `json:"rage_resist_pct,omitempty"`       // % reduction to incoming damage while raging
	ExtraActions       int           `json:"extra_actions,omitempty"`         // granted by Action Surge / Flurry — extra attack actions this turn
	Ambushing          bool          `json:"ambushing,omitempty"`             // Opened with a sneak attack: advantage on attacks this first turn
	PendingSneakDice   string        `json:"pending_sneak_dice,omitempty"`    // rogue Sneak Attack rider ("2d6") applied to the next hit
	AbilitiesUsed      []string      `json:"abilities_used,omitempty"`        // once-per-combat abilities already spent this fight
	// Helped: an ally used Help on you — your next attack has advantage.
	Helped bool `json:"helped,omitempty"`
	// Spell effects on this combatant: buffs from their own spells or an
	// ally's, and temporary hit points, which soak damage before HP does.
	SpellBuffs []SpellBuff `json:"spell_buffs,omitempty"`
	TempHP     int         `json:"temp_hp,omitempty"`
}

// MonsterInstance is a live monster in the current combat encounter
type MonsterInstance struct {
	TemplateID string           `json:"template_id"` // ID from monster JSON
	InstanceID string           `json:"instance_id"` // Unique per-combat ID ("goblin", "goblin#2", …)
	Name       string           `json:"name"`
	CurrentHP  int              `json:"current_hp"`
	MaxHP      int              `json:"max_hp"`
	ArmorClass int              `json:"armor_class"`
	Initiative int              `json:"initiative"`
	Conditions []CombatCondition `json:"conditions"`
	IsAlive    bool             `json:"is_alive"`
	ReactionUsed bool           `json:"reaction_used"` // Reaction consumed this round (OA)
	Disengaged   bool           `json:"disengaged"`    // Monster used Disengage this turn
	Fled         bool           `json:"fled"`          // Escaped off the grid edge — out of the fight, no kill
	// WaitingOnDowned: it has stopped attacking a downed player and is standing
	// over them (said once, so the log doesn't repeat it every round).
	WaitingOnDowned bool `json:"waiting_on_downed,omitempty"`
	Surprised    bool           `json:"surprised,omitempty"` // Caught unaware by a sneak attack: loses its first turn
	Pos          Position       `json:"pos"`           // Grid cell
	// SpawnPos is set only for the opening response of an encounter when this
	// monster won initiative and moved before the player ever saw the board, so
	// the client can animate the step from where it appeared. Cleared via
	// combat.ClearSpawnPositions once the opening response has been built.
	SpawnPos *Position   `json:"-"`
	Data     MonsterData `json:"data"` // Full stat block
	// Threat is damage taken from each party member (by member id): monsters
	// favour whoever hurt them most, so a tank can hold their attention.
	Threat map[string]int `json:"-"`
}

// Active reports whether the monster is still a participant (alive and on the grid).
func (m *MonsterInstance) Active() bool {
	return m.IsAlive && !m.Fled
}

// PartyCombatant is one player (or future companion) in combat. A party fight
// has several; each request acts as one of them (CombatSession.Actor).
type PartyCombatant struct {
	Type               string            `json:"type"` // "player", future: "companion"
	ID                 string            `json:"id"`   // npub
	SaveID             string            `json:"save_id"`
	Name               string            `json:"name"` // the character's name
	IsPlayerControlled bool              `json:"is_player_controlled"`
	Pos                Position          `json:"pos"` // Grid cell
	CombatState        PlayerCombatState `json:"combat_state"`

	// Per-member tallies: each player earns and keeps their own.
	Fled           bool                `json:"fled,omitempty"`  // escaped the fight (out of it, keeps XP so far)
	Ended          bool                `json:"ended,omitempty"` // has collected their outcome (loot screen closed)
	XPEarned       int                 `json:"xp_earned"`
	Loot           []LootDrop          `json:"loot,omitempty"`
	LevelUpPending bool                `json:"level_up_pending"`
	AmmoUsed       int                 `json:"ammo_used"`
	DamageTaken    int                 `json:"damage_taken"` // this fight — caps healing XP (no overheal farming)
	Concentration  *ConcentrationState `json:"concentration,omitempty"`
}

// Standing reports whether the member is still up and in the fight.
func (p *PartyCombatant) Standing() bool {
	return !p.Fled && !p.CombatState.IsUnconscious
}

// InitiativeEntry records a combatant's position in the initiative order
type InitiativeEntry struct {
	ID         string `json:"id"`         // npub for player, instance_id for monster
	Type       string `json:"type"`       // "player" or "monster"
	Name       string `json:"name"`       // display name for the turn-order strip
	Initiative int    `json:"initiative"`
	DEXScore   int    `json:"dex_score"` // For tie-breaking
}

// ConcentrationState tracks a spell the player is concentrating on during combat.
// Taking damage forces a Constitution save to keep it; failing drops the linked
// ActiveEffect. Never persisted — it lives only inside the memory-only session.
type ConcentrationState struct {
	SpellID   string `json:"spell_id"`
	SpellName string `json:"spell_name"`
	EffectID  string `json:"effect_id"` // ActiveEffect to remove if concentration breaks ("" if none)
}

// CombatSession holds all in-memory state for an active combat encounter.
// This is NEVER written to the save file — it lives only in GameSession memory.
type CombatSession struct {
	Party              []PartyCombatant  `json:"party"`
	Monsters           []MonsterInstance `json:"monsters"`
	Initiative         []InitiativeEntry `json:"initiative"`
	CurrentTurnIndex   int               `json:"current_turn_index"` // index into Initiative of whoever is acting
	Round              int               `json:"round"`              // increments each time the turn order wraps
	GridWidth          int               `json:"grid_width"`   // Sized by combatant count (combat.GridSizeFor)
	GridHeight         int               `json:"grid_height"`  // Sized by combatant count (combat.GridSizeFor)
	Log                []string          `json:"log"`
	EnvironmentID      string            `json:"environment_id"`
	Battlefield        string            `json:"battlefield,omitempty"` // terrain profile: "forest", "interior", "cellar", …
	Terrain            []string          `json:"terrain,omitempty"`     // row-major GridWidth×GridHeight terrain ids; "" = open ground
	IsSurprised        bool              `json:"is_surprised"`  // Party was surprised (every monster acts before the party in round 1)
	Phase              string            `json:"phase"`         // "active", "loot", "victory", "defeat", "death_saves"

	// Actor is the party member the current request acts as (an index into
	// Party). Set per request by the API layer, and to a monster's target
	// during its turn. Always 0 in a solo fight.
	Actor int `json:"-"`
	// TurnDeadline (unix ms) ends a party member's turn: when it passes they
	// take the Dodge action. 0 in solo fights (no timer).
	TurnDeadline int64 `json:"turn_deadline,omitempty"`
	// Retreat: the leader called a retreat and it succeeded — everyone left.
	Retreated bool `json:"retreated,omitempty"`

	// Difficulty rates the fight against the player's level band at combat start
	// ("trivial"…"deadly", see encounter.Difficulty). Surfaced so the client can
	// warn on a fight that outclasses the player (M5 §22). Set once, memory-only.
	Difficulty string `json:"difficulty,omitempty"`

}

// PendingEncounter is an encounter whose monsters haven't noticed the player
// yet: the fight hasn't started, and the player chooses to sneak away, sneak
// attack, or approach (party-combat-plan §7). Server memory only.
type PendingEncounter struct {
	MonsterIDs    []string `json:"monster_ids"`
	EnvironmentID string   `json:"environment_id"`
}
