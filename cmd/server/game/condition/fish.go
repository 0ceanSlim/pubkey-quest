package condition

// KeyLb is a landed fish's exact weight in pounds (fishing-system.md §2): set
// when it's caught, shown in its tooltip and on the catch card. It's cosmetic —
// price and carry weight come from the size band — but it can't be derived, so
// it travels with the slot like any other condition key.
const KeyLb = "lb"

func init() { StateKeys = append(StateKeys, KeyLb) }
