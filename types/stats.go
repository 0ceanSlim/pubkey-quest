package types

// ScoreCap is the most an ability score or a skill can be: 20, whatever
// points, gear, spells or effects add (beta exploit audit: Charisma past 20
// bent shop prices). Penalties still take a score below its base.
const ScoreCap = 20

// CapScore holds an ability score or skill value to ScoreCap.
func CapScore(n int) int {
	if n > ScoreCap {
		return ScoreCap
	}
	return n
}
