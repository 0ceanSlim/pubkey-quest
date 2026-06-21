package types

// QuestObjectiveType is the kind of action a player must take to satisfy a stage objective.
type QuestObjectiveType string

const (
	ObjectiveTalk    QuestObjectiveType = "talk"    // Speak with a specific NPC
	ObjectiveFetch   QuestObjectiveType = "fetch"   // Acquire an item
	ObjectiveExplore QuestObjectiveType = "explore" // Reach a location, POI, or sub-area
	ObjectiveSlay    QuestObjectiveType = "slay"    // Defeat monsters of a given ID
	ObjectiveCheck   QuestObjectiveType = "check"   // Pass a skill check at a specific DC
)

// QuestObjective is one task within a quest stage.
//
// Field usage by objective type:
//
//	talk     Target=npc-id
//	fetch    Target=item-id, Count (default 1)
//	explore  Target=location-id or poi-id
//	slay     Target=monster-id, Count (default 1)
//	check    Skill=skill-id, Value=DC
type QuestObjective struct {
	Type        QuestObjectiveType `json:"type"`
	Target      string             `json:"target,omitempty"`
	Skill       string             `json:"skill,omitempty"`
	Count       int                `json:"count,omitempty"`
	Value       int                `json:"value,omitempty"`
	Description string             `json:"description"`
}

// QuestStage is one milestone in a quest line.
//
// WaitMinutes >0 means the stage doesn't become active until the in-game
// clock advances by that amount after the previous stage completes (used
// for "come back tomorrow" pacing).
type QuestStage struct {
	ID          string           `json:"id"`
	Description string           `json:"description"`
	WaitMinutes int              `json:"wait_minutes,omitempty"`
	Objectives  []QuestObjective `json:"objectives"`
	Rewards     *POIReward       `json:"rewards,omitempty"`
	UnlocksPOI  string           `json:"unlocks_poi,omitempty"`
}

// QuestStartCondition tells the player how to begin the quest.
type QuestStartCondition struct {
	Type      string `json:"type"` // "talk", "explore", "item", "bulletin_board"
	Target    string `json:"target"`
	Location  string `json:"location,omitempty"`
	StartHint string `json:"start_hint"`
}

// QuestRecommendation is informational guidance shown in the quest log.
type QuestRecommendation struct {
	RecommendedStats []string `json:"recommended_stats,omitempty"`
	CombatDanger     string   `json:"combat_danger,omitempty"`
}

// QuestCategory groups quests in the log and on disk.
type QuestCategory string

const (
	QuestMain   QuestCategory = "main"
	QuestSide   QuestCategory = "side"
	QuestClass  QuestCategory = "class"
	QuestRace   QuestCategory = "race"
	QuestDaily  QuestCategory = "daily"
	QuestWeekly QuestCategory = "weekly"
)

// QuestData is the full configuration for a hand-authored quest.
//
// Prerequisites is a list of quest IDs that must be completed before this
// quest becomes available; gating beyond that (level, class, items, etc.)
// goes through Requirements using the shared POIRequirement schema.
//
// IsRandomized=true marks dailies/weeklies whose stages may be re-rolled.
type QuestData struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Category       QuestCategory        `json:"category"`
	Difficulty     string               `json:"difficulty"`
	StepCount      int                  `json:"step_count"`
	TotalQP        int                  `json:"total_qp"`
	IsRandomized   bool                 `json:"is_randomized,omitempty"`
	Description    string               `json:"description"`
	StartCondition QuestStartCondition  `json:"start_condition"`
	Requirements   []POIRequirement     `json:"requirements,omitempty"`
	Prerequisites  []string             `json:"prerequisites,omitempty"`
	Recommended    *QuestRecommendation `json:"recommended,omitempty"`
	Stages         []QuestStage         `json:"stages"`
}

// PlayerQuestProgress tracks a single player's state in one quest.
//
// Clock semantics: ReadyAtDay/ReadyAtMinute (wait stages) use the in-game
// clock. LastRolledDay/LastRolledWeek (daily/weekly resets) are real-world
// day/week indices — dailies and weeklies reset on real time, not game time.
type PlayerQuestProgress struct {
	QuestID          string         `json:"quest_id"`
	CurrentStage     int            `json:"current_stage"`
	Completed        bool           `json:"completed"`
	ReadyAtMinute    int            `json:"ready_at_minute,omitempty"`
	ReadyAtDay       int            `json:"ready_at_day,omitempty"`
	LastRolledDay    int            `json:"last_rolled_day,omitempty"`
	LastRolledWeek   int            `json:"last_rolled_week,omitempty"`
	ObjectiveTracker map[string]int `json:"objective_tracker,omitempty"`
}
