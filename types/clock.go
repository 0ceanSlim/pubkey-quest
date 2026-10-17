package types

// The night window, in minutes since midnight: 23:00 until 05:00. Combat pays
// more XP inside it (combat.IsNight), and the client announces nightfall and
// dawn at its edges (game-data "clock").
const (
	NightStart = 1380
	NightEnd   = 300
)
