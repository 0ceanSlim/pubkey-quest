package combat

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// RollD20 returns a random d20 roll (1–20)
func RollD20() int {
	return rand.Intn(20) + 1
}

// RollD returns a random roll of an N-sided die
func RollD(sides int) int {
	if sides <= 0 {
		return 0
	}
	return rand.Intn(sides) + 1
}

// ParseDice parses a dice string like "2d6" into count and sides.
// For versatile weapons like "1d8,1d10", pass the substring you want first.
func ParseDice(dice string) (int, int, error) {
	dice = strings.TrimSpace(strings.ToLower(dice))
	// Take only the first variant for versatile weapons (e.g., "1d8,1d10" → "1d8")
	if idx := strings.Index(dice, ","); idx >= 0 {
		dice = dice[:idx]
	}
	parts := strings.SplitN(dice, "d", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid dice format: %s", dice)
	}

	count, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid dice count in %q: %v", dice, err)
	}

	sides, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid dice sides in %q: %v", dice, err)
	}

	return count, sides, nil
}

// ParseVersatileDice returns the two-handed dice string from a versatile weapon.
// Returns the original string if not versatile.
func ParseVersatileDice(damage string) (oneHand, twoHand string) {
	parts := strings.SplitN(damage, ",", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return damage, damage
}

// RollDice rolls the given dice expression and returns the total.
// On a critical hit, the dice count is doubled.
func RollDice(diceExpr string, isCrit bool) int {
	count, sides, err := ParseDice(diceExpr)
	if err != nil {
		return 1 // Fallback for malformed dice
	}

	if isCrit {
		count *= 2
	}

	total := 0
	for i := 0; i < count; i++ {
		total += RollD(sides)
	}
	return total
}

// RollAdvantage rolls two d20s and returns (result, roll1, roll2) where result is the higher
func RollAdvantage() (int, int, int) {
	r1 := RollD20()
	r2 := RollD20()
	if r1 >= r2 {
		return r1, r1, r2
	}
	return r2, r1, r2
}

// RollDisadvantage rolls two d20s and returns (result, roll1, roll2) where result is the lower
func RollDisadvantage() (int, int, int) {
	r1 := RollD20()
	r2 := RollD20()
	if r1 <= r2 {
		return r1, r1, r2
	}
	return r2, r1, r2
}

// StatMod returns the D&D ability score modifier for the given stat value.
// Uses floor division to match D&D rules (e.g., STR 9 → -1, STR 10 → +0).
func StatMod(stat int) int {
	diff := stat - 10
	if diff < 0 && diff%2 != 0 {
		return (diff - 1) / 2
	}
	return diff / 2
}

// GetStatFromMap safely reads an integer stat from a map[string]interface{}.
// JSON numbers parse as float64, so handles both float64 and int.
func GetStatFromMap(stats map[string]interface{}, key string) int {
	if v, ok := stats[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		}
	}
	return 10 // D&D default
}

// RollRange returns a random value between min and max (inclusive)
func RollRange(min, max int) int {
	if min == max {
		return min
	}
	if min > max {
		min, max = max, min
	}
	return min + rand.Intn(max-min+1)
}
