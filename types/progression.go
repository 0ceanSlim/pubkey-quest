package types

// LevelUpResult is returned by the central XP-grant path. Leveled is true only
// when the granted XP crossed one or more advancement thresholds. It carries
// old→new values so any action that awards XP (combat, performance, quest,
// exploration) can surface a level-up moment to the UI without each caller
// re-deriving the numbers.
type LevelUpResult struct {
	GainedXP   int  `json:"gained_xp"`
	Leveled    bool `json:"leveled"`
	OldLevel   int  `json:"old_level"`
	NewLevel   int  `json:"new_level"`
	OldMaxHP   int  `json:"old_max_hp"`
	NewMaxHP   int  `json:"new_max_hp"`
	OldMaxMana int  `json:"old_max_mana"`
	NewMaxMana int  `json:"new_max_mana"`
}
