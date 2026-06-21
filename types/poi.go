package types

// POIStepType represents the type of interaction at a POI/encounter step.
//
// All POI and encounter content is represented as a directed graph of nodes;
// every node has exactly one of these types.
type POIStepType string

const (
	POIStepNarrative      POIStepType = "narrative"       // Pure text node, advances via Next
	POIStepChoice         POIStepType = "choice"          // Branching node with player-selected Choices
	POIStepCheck          POIStepType = "check"           // Active skill check (player rolls)
	POIStepPassiveCheck   POIStepType = "passive_check"   // Hidden skill check (no player input)
	POIStepMonster        POIStepType = "monster"         // Combat encounter; advances via Next on victory
	POIStepLoot           POIStepType = "loot"            // Reward distribution from a loot table
	POIStepDamage         POIStepType = "damage"          // Health penalty
	POIStepEffect         POIStepType = "effect"          // Status effect application
	POIStepReward         POIStepType = "reward"          // XP/gold/quest-point grant (no item drops)
	POIStepTransaction    POIStepType = "transaction"     // Spend resources (gold/items) for a reward
	POIStepExit           POIStepType = "exit"            // Terminal node (use IsTerminal on any node, but exit is explicit)
	POIStepNPCInteraction POIStepType = "npc_interaction" // Hand off to NPC dialogue system
)

// POIRequirement gates a node, choice, or whole POI/encounter/quest.
//
// Numeric thresholds (skill, stat, level, quest_points) use Min.
// Enumerated requirements (class, race, alignment) use Values.
// Item references use ID and optionally Consumed.
//
// Examples:
//
//	{"type":"skill","id":"athletics","min":14}
//	{"type":"stat","id":"strength","min":12}
//	{"type":"class","values":["Druid","Ranger"]}
//	{"type":"alignment","values":["good","neutral_good","lawful_good"]}
//	{"type":"item","id":"silver-key","consumed":true}
//	{"type":"level","min":5}
//	{"type":"quest_points","min":10}
//	{"type":"quest_completed","id":"the-rising-shadow"}
type POIRequirement struct {
	Type        string   `json:"type"`
	ID          string   `json:"id,omitempty"`
	Min         int      `json:"min,omitempty"`
	Values      []string `json:"values,omitempty"`
	Consumed    bool     `json:"consumed,omitempty"`
	Description string   `json:"description,omitempty"`
}

// POIChoice is a player-selectable option on a choice node.
type POIChoice struct {
	Label        string           `json:"label"`
	Next         string           `json:"next"`
	Requirements []POIRequirement `json:"requirements,omitempty"`
}

// POIDamage is the payload for a damage node.
type POIDamage struct {
	Type   string `json:"type"` // "bludgeoning", "piercing", "fire", etc.
	Amount int    `json:"amount"`
}

// POIEffect is the payload for an effect node, or a reward bundled with a buff.
type POIEffect struct {
	ID              string `json:"id"`
	Amount          int    `json:"amount,omitempty"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
}

// POIRewardItem is a single item entry inside a reward.
type POIRewardItem struct {
	ID       string `json:"id"`
	Quantity int    `json:"quantity"`
}

// POIReward is the payload for a reward node, transaction node, or quest stage payout.
type POIReward struct {
	XP          int             `json:"xp,omitempty"`
	Gold        int             `json:"gold,omitempty"`
	QuestPoints int             `json:"quest_points,omitempty"`
	Items       []POIRewardItem `json:"items,omitempty"`
	Effect      *POIEffect      `json:"effect,omitempty"`
}

// POICost is the resource cost paid by a transaction node.
type POICost struct {
	Gold  int             `json:"gold,omitempty"`
	Items []POIRewardItem `json:"items,omitempty"`
}

// POIStep is a single node in the interaction graph.
//
// Field usage by node type:
//
//	narrative       Text, Next
//	choice          Text, Choices
//	check           Text, Skill, DC, SuccessText, SuccessNext, FailureText, FailureNext
//	passive_check   Skill, DC, SuccessNext, FailureNext (no player-visible text)
//	monster         Text, MonsterID, Count, Surprise, Next
//	loot            Text, LootTable, Next or IsTerminal
//	damage          Text, Damage, Next
//	effect          Text, Effect, Next
//	reward          Text, Reward, Next or IsTerminal
//	transaction     Text, Cost, Reward, Next or IsTerminal
//	exit            Text, IsTerminal=true (terminal nodes can use IsTerminal on any type)
//	npc_interaction Text, NPCIDs, IsTerminal=true (handover)
type POIStep struct {
	ID           string           `json:"id,omitempty"` // Optional; map key is canonical
	Type         POIStepType      `json:"type"`
	Text         string           `json:"text,omitempty"`
	SuccessText  string           `json:"success_text,omitempty"`
	FailureText  string           `json:"failure_text,omitempty"`
	Skill        string           `json:"skill,omitempty"`
	DC           int              `json:"dc,omitempty"`
	MonsterID    string           `json:"monster_id,omitempty"`
	Count        int              `json:"count,omitempty"` // monster count
	Surprise     bool             `json:"surprise,omitempty"`
	LootTable    *POILootTable    `json:"loot_table,omitempty"`
	Choices      []POIChoice      `json:"choices,omitempty"`
	Next         string           `json:"next,omitempty"`
	SuccessNext  string           `json:"success_next,omitempty"`
	FailureNext  string           `json:"failure_next,omitempty"`
	IsTerminal   bool             `json:"is_terminal,omitempty"`
	Damage       *POIDamage       `json:"damage,omitempty"`
	Effect       *POIEffect       `json:"effect,omitempty"`
	Reward       *POIReward       `json:"reward,omitempty"`
	Cost         *POICost         `json:"cost,omitempty"`
	Requirements []POIRequirement `json:"requirements,omitempty"`
	NPCIDs       []string         `json:"npc_ids,omitempty"`
}

// POILootTable describes drops at a loot node.
//
// Guaranteed entries always drop (quantity may be a [min,max] range expressed
// as a 2-element array in JSON; see loader).
// Tiered entries roll Rolls times against weighted tier buckets.
type POILootTable struct {
	Guaranteed []POILootEntry `json:"guaranteed,omitempty"`
	Rolls      int            `json:"rolls,omitempty"`
	Tiers      []POILootTier  `json:"tiers,omitempty"`
}

// POILootEntry is one possible drop. Quantity is a JSON number or [min,max] array.
type POILootEntry struct {
	Item     string      `json:"item"`
	Quantity any    `json:"quantity"`
	Weight   int         `json:"weight,omitempty"` // only meaningful inside a tier
}

// POILootTier is a weighted bucket of entries; one entry is rolled per Rolls.
type POILootTier struct {
	Name    string         `json:"name"`
	Weight  int            `json:"weight"`
	Entries []POILootEntry `json:"entries"`
}

// POIDiscovery defines how a POI is found while travelling.
type POIDiscovery struct {
	Chance  float64 `json:"chance"`            // 0.0-1.0 base discovery probability
	Skill   string  `json:"skill,omitempty"`   // optional perception-style check that boosts/replaces chance
	DC      int     `json:"dc,omitempty"`      // DC for the optional skill check
	Message string  `json:"message"`           // shown on discovery
}

// POICategory is a labelling tag (drives icon/UI flavour, not behaviour).
type POICategory string

const (
	POIDungeon    POICategory = "dungeon"
	POILandmark   POICategory = "landmark"
	POIUtility    POICategory = "utility"
	POISettlement POICategory = "settlement"
)

// POIData is the full configuration for a discoverable point of interest.
//
// All POIs use the same node-graph schema; Category is purely a label for UI.
// POI NPCs live in game-data/npcs/<poi_id>/<npc>.json and are referenced
// by NPCIDs. Encounter NPCs (which are always static and self-contained) are
// the only NPCs still embedded inline; see EncounterData.NPCs.
type POIData struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Category          POICategory        `json:"category"`
	ParentEnvironment string             `json:"parent_environment"`
	Position          float64            `json:"position"`
	Description       string             `json:"description"`
	Discovery         POIDiscovery       `json:"discovery"`
	Requirements      []POIRequirement   `json:"requirements,omitempty"`
	StartNode         string             `json:"start_node"`
	Nodes             map[string]POIStep `json:"nodes"`
	NPCIDs            []string           `json:"npc_ids,omitempty"`
}

// EncounterTrigger defines when an encounter can fire.
type EncounterTrigger string

const (
	EncounterTriggerTravel       EncounterTrigger = "travel"        // Fires while travelling between locations
	EncounterTriggerLocation     EncounterTrigger = "location"      // Fires on entering specific locations (ValidLocations)
	EncounterTriggerBuilding     EncounterTrigger = "building"      // Fires on entering specific buildings (ValidLocations holds building IDs)
	EncounterTriggerBuildingType EncounterTrigger = "building_type" // Fires on entering any building of given types (BuildingTypes)
)

// EncounterData is a triggered narrative event.
//
// Encounters share the POI step graph but differ in trigger semantics:
// they fire probabilistically when the player enters a context, are usually
// transient, and may be repeatable with a cooldown. POIs by contrast are
// permanent map locations that are discovered once and revisitable.
type EncounterData struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Trigger         EncounterTrigger   `json:"trigger"`
	ValidLocations  []string           `json:"valid_locations,omitempty"` // for location/building triggers
	BuildingTypes   []string           `json:"building_types,omitempty"`  // for building_type trigger
	Chance          float64            `json:"chance"`                    // 0.0-1.0 per check
	Repeatable      bool               `json:"repeatable,omitempty"`
	CooldownMinutes int                `json:"cooldown_minutes,omitempty"`
	Requirements    []POIRequirement   `json:"requirements,omitempty"`
	StartNode       string             `json:"start_node"`
	Nodes           map[string]POIStep `json:"nodes"`
	NPCs            []NPCData          `json:"npcs,omitempty"`
}
