package types

// The night window, in minutes since midnight: 23:00 until 05:00. Combat pays
// more XP inside it (combat.IsNight), it's dark outdoors (attacks without
// darkvision or a light are at disadvantage), road encounters are more likely,
// and the client announces nightfall and dawn at its edges (game-data "clock").
const (
	NightStart = 1380
	NightEnd   = 300
)

// IsNight reports whether a time of day (minutes since midnight) is in the
// night window.
func IsNight(timeOfDay int) bool {
	return InWindow(timeOfDay, NightStart, NightEnd)
}

// InWindow reports whether a time of day falls in [from, to), a window that
// may wrap past midnight (21:00-09:00 is from 1260 to 540).
func InWindow(timeOfDay, from, to int) bool {
	t := ((timeOfDay % 1440) + 1440) % 1440
	if from <= to {
		return t >= from && t < to
	}
	return t >= from || t < to
}

// WeekDays are the names of the 7-day week, from day 1 of a new character.
// NPC schedule entries may name the days they apply on ("days": ["restday"]).
var WeekDays = [7]string{"Firstday", "Toilday", "Midweek", "Hearthday", "Fairday", "Marketday", "Restday"}

// Weekday is the day of the week of a save's CurrentDay (day 1 is Firstday).
func Weekday(currentDay int) string {
	return WeekDays[(((currentDay-1)%7)+7)%7]
}
