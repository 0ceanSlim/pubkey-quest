package types

// SurvivalConfig is game-data/systems/survival.json: the wild's cold and heat
// (docs/draft/survival-gear.md). Each is an hourly Constitution save while
// you're out in it; a failure costs fatigue.
type SurvivalConfig struct {
	Cold Exposure `json:"cold"`
	Heat Exposure `json:"heat"`
	// Whistle is the signal whistle's item id.
	Whistle string `json:"whistle"`
}

// Exposure is one kind of weather: where it bites, how hard, and what keeps
// it off.
type Exposure struct {
	Name         string   `json:"name"`         // "cold", "heat"
	Environments []string `json:"environments"` // environment ids
	// From and To are the hours it bites, [from, to); both 0 means all day.
	From int `json:"from,omitempty"`
	To   int `json:"to,omitempty"`
	DC   int `json:"dc"`
	// DCPerDryHour raises the DC for each hour in it since your last drink.
	DCPerDryHour int `json:"dc_per_dry_hour,omitempty"`
	// Water: a drink keeps it off while its "watered" effect lasts, and a
	// carried waterskin is drunk from when a save comes due.
	Water bool `json:"water,omitempty"`
	// Resist is the damage type whose resistance keeps it off.
	Resist string `json:"resist,omitempty"`
	// Protect: carrying any of these keeps it off.
	Protect []string `json:"protect,omitempty"`
	// Advantage: carrying any of these gives advantage on the save.
	Advantage []string `json:"advantage,omitempty"`
	// Fire: asleep in the wild with one of these, it's kept off all night.
	Fire []string `json:"fire,omitempty"`
	// Disadvantage: chest armour of these types gives disadvantage.
	Disadvantage []string `json:"disadvantage,omitempty"`
	Fatigue      int      `json:"fatigue"` // per failed save
	Effect       string   `json:"effect"`  // shown while you're out in it unprotected
	Fail         string   `json:"fail"`    // the log line on a failed save
}
