package gameutil

import "pubkey-quest/types"

// Room rentals (M2): a paid room is recorded on the save (types.Rental) so it
// survives a reload, and unlocks the building's rented-state room. Rentals are a
// non-derivable paid outcome, so they belong in the save per the §4 hydration rule.

// HasActiveRental reports whether the player holds an unexpired rental for a building.
func HasActiveRental(state *types.SaveFile, buildingID string) bool {
	for _, r := range state.Rentals {
		if r.Building != buildingID {
			continue
		}
		if r.ExpiresDay > state.CurrentDay || (r.ExpiresDay == state.CurrentDay && r.ExpiresMin > state.TimeOfDay) {
			return true
		}
	}
	return false
}

// AddRental records a paid room rental on the save, replacing any prior rental
// for the same building.
func AddRental(state *types.SaveFile, buildingID string, expiresDay, expiresMin int) {
	var kept []types.Rental
	for _, r := range state.Rentals {
		if r.Building != buildingID {
			kept = append(kept, r)
		}
	}
	kept = append(kept, types.Rental{Building: buildingID, ExpiresDay: expiresDay, ExpiresMin: expiresMin})
	state.Rentals = kept
}

// EndRentalBy brings a building's rental to an end no later than day/minute:
// a night's sleep ends it at the morning check-out. A rental that already ends
// sooner is left alone.
func EndRentalBy(state *types.SaveFile, buildingID string, day, minute int) {
	for i, r := range state.Rentals {
		if r.Building != buildingID {
			continue
		}
		if r.ExpiresDay > day || (r.ExpiresDay == day && r.ExpiresMin > minute) {
			state.Rentals[i].ExpiresDay, state.Rentals[i].ExpiresMin = day, minute
		}
	}
}
