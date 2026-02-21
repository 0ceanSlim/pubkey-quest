package session

import (
	"pubkey-quest/types"
)

// GameSession holds the in-memory game state for an active session
type GameSession struct {
	Npub      string         `json:"npub"`
	SaveID    string         `json:"save_id"`
	SaveData  types.SaveFile `json:"save_data"`
	LoadedAt  int64          `json:"loaded_at"`
	UpdatedAt int64          `json:"updated_at"`

	// Session-only data (not persisted to save files)
	BookedShows    []map[string]interface{} `json:"booked_shows,omitempty"`    // Current show bookings
	PerformedShows []string                 `json:"performed_shows,omitempty"` // Shows performed today (to prevent re-booking)
	RentedRooms    []map[string]interface{} `json:"rented_rooms,omitempty"`    // Current room rentals

	// Combat state — lives in server memory only, never written to save files.
	// Nil when no combat is in progress.
	ActiveCombat *types.CombatSession `json:"-"`

	// Auto-pause tracking: tracks time since last player action
	LastActionTime     int64 `json:"-"` // Real-time timestamp of last player action
	LastActionGameTime int   `json:"-"` // In-game time (TimeOfDay) of last player action

	// Delta system: cached state for surgical updates
	LastSnapshot       *SessionSnapshot `json:"-"` // Previous state for delta calculation
	NPCsAtLocation     []string         `json:"-"` // Cached NPCs at current location
	NPCsLastHour       int              `json:"-"` // Hour when NPCs were last fetched
	BuildingStates     map[string]bool  `json:"-"` // Cached building open/close states
	BuildingsLastCheck int              `json:"-"` // Time when buildings were last checked
}

// SessionSnapshot captures state at a point in time for delta calculation
type SessionSnapshot struct {
	// Character stats
	HP         int
	MaxHP      int
	Mana       int
	MaxMana    int
	Fatigue    int
	Hunger     int
	Gold       int
	XP         int
	TimeOfDay  int
	CurrentDay int

	// Location
	City     string
	District string
	Building string

	// NPCs and buildings at current location
	NPCs      []string
	Buildings map[string]bool // building_id -> isOpen

	// Inventory (general slots - 4 slots)
	GeneralSlots [4]InventorySlotSnapshot

	// Backpack (20 slots)
	BackpackSlots [20]InventorySlotSnapshot

	// Equipment
	EquipmentSlots map[string]string // slot_name -> item_id

	// Active effects
	ActiveEffects []string // Effect IDs

	// Show readiness (for Play Show button)
	ShowReady         bool   // Whether a show is currently ready to perform
	ShowReadyBuilding string // Building ID where the show is ready
}

// InventorySlotSnapshot captures state of a single inventory slot
type InventorySlotSnapshot struct {
	ItemID   string
	Quantity int
}

// GetSaveData returns a pointer to the session's save data
func (s *GameSession) GetSaveData() *types.SaveFile {
	return &s.SaveData
}

// GetBookedShows returns the session's booked shows
func (s *GameSession) GetBookedShows() []map[string]interface{} {
	return s.BookedShows
}

// SetBookedShows sets the session's booked shows
func (s *GameSession) SetBookedShows(shows []map[string]interface{}) {
	s.BookedShows = shows
}

// GetRentedRooms returns the session's rented rooms
func (s *GameSession) GetRentedRooms() []map[string]interface{} {
	return s.RentedRooms
}

// SetRentedRooms sets the session's rented rooms
func (s *GameSession) SetRentedRooms(rooms []map[string]interface{}) {
	s.RentedRooms = rooms
}

// GetPerformedShows returns the session's performed shows
func (s *GameSession) GetPerformedShows() []string {
	return s.PerformedShows
}

// SetPerformedShows sets the session's performed shows
func (s *GameSession) SetPerformedShows(shows []string) {
	s.PerformedShows = shows
}

// GetLastActionGameTime returns the in-game time of last player action
func (s *GameSession) GetLastActionGameTime() int {
	return s.LastActionGameTime
}

// SetLastActionTime sets the real-time timestamp of last player action
func (s *GameSession) SetLastActionTime(unixTime int64) {
	s.LastActionTime = unixTime
}

// SetLastActionGameTime sets the in-game time of last player action
func (s *GameSession) SetLastActionGameTime(gameTime int) {
	s.LastActionGameTime = gameTime
}

// GetSaveDataTimeOfDay returns the current time of day from save data
func (s *GameSession) GetSaveDataTimeOfDay() int {
	return s.SaveData.TimeOfDay
}

// GetNPCsAtLocation returns the cached NPCs at current location
func (s *GameSession) GetNPCsAtLocation() []string {
	return s.NPCsAtLocation
}

// UpdateNPCsAtLocation updates the cached NPCs and hour
func (s *GameSession) UpdateNPCsAtLocation(npcs []string, currentHour int) {
	s.NPCsAtLocation = npcs
	s.NPCsLastHour = currentHour
}

// UpdateBuildingStates updates the cached building states
func (s *GameSession) UpdateBuildingStates(buildings map[string]bool, currentTime int) {
	if s.BuildingStates == nil {
		s.BuildingStates = make(map[string]bool)
	}
	for k, v := range buildings {
		s.BuildingStates[k] = v
	}
	s.BuildingsLastCheck = currentTime
}

// ShouldRefreshNPCs returns true if NPCs should be refreshed (hour changed)
func (s *GameSession) ShouldRefreshNPCs(currentHour int) bool {
	return currentHour != s.NPCsLastHour || len(s.NPCsAtLocation) == 0
}

// ShouldRefreshBuildings returns true if buildings should be refreshed (every 5 in-game minutes or first call)
func (s *GameSession) ShouldRefreshBuildings(currentTime int) bool {
	if s.BuildingsLastCheck == 0 {
		return true // First call
	}
	return currentTime-s.BuildingsLastCheck >= 5
}
