package session

import (
	"sync"

	"pubkey-quest/cmd/server/game/locks"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/world"
	"pubkey-quest/types"
)

// GameSession holds the in-memory game state for an active session
type GameSession struct {
	// mu serializes requests that change this session (Lock/Unlock). Party
	// moves never lock another player's session: they queue PartySyncs, which
	// that player's own next request applies under its own lock.
	mu sync.Mutex

	// PartySyncs is the queue of leader moves waiting to be applied to this
	// (follower) session, guarded by its own tiny mutex.
	syncMu     sync.Mutex
	partySyncs []PartySync

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

	// PendingSave is a save prepared for signing (relay saves): the exact
	// event content the player must sign, and the disk copy to write once a
	// relay accepts it. Memory only.
	PendingSave *PendingSave `json:"-"`

	// SellStaged holds items taken out of the pack onto a shop's sell counter
	// (item id → quantity). Only these can be sold or handed back, so the shop
	// never pays for, or returns, an item the server didn't take. Memory only;
	// returned to the pack before any save.
	SellStaged map[string]int `json:"-"`

	// PendingEncounter is an encounter whose monsters haven't noticed the
	// player: no fight yet, waiting on their choice (POST /api/combat/approach).
	// Memory only; nil when nothing is waiting.
	PendingEncounter *types.PendingEncounter `json:"-"`

	// POI walk state — lives in server memory only, never written to save files.
	// Nil when the player is not inside a discovered POI. A monster node inside a
	// POI sets ActiveCombat too and stashes the resume node here (poi.Session).
	ActivePOI *poi.Session `json:"-"`

	// Ground holds items dropped on the ground per location — server-authoritative,
	// memory-only. Drop moves an item here; pickup takes it back. Nil-safe.
	Ground *world.GroundStore `json:"-"`

	// POINPCs are the people a POI walk just introduced (Thistle at her wagon):
	// they have no schedule in a city, so talking to them is allowed only while
	// the player is still where they met (POINPCPlace + POINPCProgress).
	POINPCs        map[string]bool `json:"-"`
	POINPCPlace    string          `json:"-"`
	POINPCProgress float64         `json:"-"`

	// Locks remembers which locks this player picked open or jammed today
	// (memory only; see game/locks).
	Locks locks.Memory `json:"-"`
	// LockSession is the lock being picked as a minigame right now (nil when
	// not picking). Any other action abandons it.
	LockSession *locks.Session `json:"-"`

	// Travel-encounter cooldown: the absolute in-game minute (day*1440 +
	// time_of_day) of the last biome encounter, so they can't fire back-to-back.
	// Session-only.
	LastEncounterTime int `json:"-"`

	// LastEncounterMonster is the previous biome-encounter monster id, so the
	// roller can avoid an immediate repeat when the biome has alternatives.
	// Session-only.
	LastEncounterMonster string `json:"-"`

	// Authored-encounter firing log: encounter id → absolute in-game minute it
	// last fired, for per-encounter cooldown + non-repeatable one-shots. Session-
	// only (M1: LastFired lives in memory, not the save).
	EncountersFired map[string]int `json:"-"`

	// Where the player is standing and when they got there, in absolute in-game
	// minutes. Event-driven encounters (a district pickpocket, a tavern brawler)
	// roll once per arrival after a short dwell, so they read as "something
	// happened while you were here" instead of ambushing the doorway — and so
	// pacing back and forth can't farm them. Session-only.
	PlaceKey    string `json:"-"`
	PlaceSince  int    `json:"-"`
	PlaceRolled bool   `json:"-"`
	// PlaceSearched: this arrival has had its one roll to notice a hidden city
	// POI (separate from PlaceRolled, the encounter roll).
	PlaceSearched bool `json:"-"`
	// PlaceInCity caches whether the current location is a city rather than a
	// travel environment, and PlaceBuildingType the current building's resolved
	// type — both looked up once per move instead of once per action.
	PlaceInCity       bool   `json:"-"`
	PlaceBuildingType string `json:"-"`

	// Presence: the normalized spot / display text published for other players,
	// recomputed only when PresenceKey (the raw place) changes. Session-only.
	PresenceKey   string `json:"-"`
	PresenceSpot  string `json:"-"`
	PresenceWhere string `json:"-"`
	PresenceArea  string `json:"-"`
	// RouteFirstEnd is the current road's first end (its connects[0]), the zero
	// point for RoutePos.
	RouteFirstEnd string `json:"-"`

	// Quest-stage encounters (a cellar full of rats): the place the last one was
	// checked at, and how many kills were still owed when it last fired there.
	// A fight starts again only when that number changes (the next wave after a
	// win) or after the player leaves and comes back (a retry after fleeing).
	QuestEncounterPlace string `json:"-"`
	QuestEncounterOwed  int    `json:"-"`

	// Spell preparation queue — lives in server memory only, cleared on session unload.
	// Each entry tracks a spell being prepared for a specific slot.
	PrepQueue []types.SpellPrepTask `json:"-"`

	// Auto-pause tracking: tracks time since last player action
	LastActionTime     int64 `json:"-"` // Real-time timestamp of last player action
	LastActionGameTime int   `json:"-"` // In-game time of last player action, absolute minutes (CurrentDay*1440 + TimeOfDay)

	// ClockRunning is the player's play/pause choice — the one source of truth
	// the play button shows. update_time only advances the world while it's on
	// (a follower's clock follows the leader's instead). Combat, POI walks and
	// encounters hold the clock in the client without changing it.
	ClockRunning bool `json:"-"`

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

// GetLastActionTime returns the real-time timestamp (unix seconds) of the last player action
func (s *GameSession) GetLastActionTime() int64 {
	return s.LastActionTime
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

// GetSaveDataAbsMinutes returns the save's absolute in-game minute.
func (s *GameSession) GetSaveDataAbsMinutes() int {
	return s.SaveData.CurrentDay*1440 + s.SaveData.TimeOfDay
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

// Lock serializes a request against this session.
func (s *GameSession) Lock() { s.mu.Lock() }

// Unlock releases the session.
func (s *GameSession) Unlock() { s.mu.Unlock() }

// PartySync is a party leader's move, queued for a follower: where the party
// now is, its clock, and its travel state (docs/draft/social-plan.md §9).
type PartySync struct {
	Location       string
	District       string
	Building       string
	Room           string
	CurrentDay     int
	TimeOfDay      int
	TravelProgress float64
	TravelStopped  bool
	// Discovered are places the leader (and so the party) just found; Notes are
	// lines for the followers' log (the leader's travel, discoveries, walks).
	Discovered []string
	Notes      []string
	// AccrueFatigue: whether the time this sync covers builds fatigue (false
	// for waiting).
	AccrueFatigue bool
	// Sleep is set when the leader slept: "inn", "camp" or "wild" — the
	// follower sleeps the same night. The sync's clock is the leader's time
	// before sleeping. Rest is the same for the old rest action.
	Sleep string
	Rest  bool
	Note  string // a log line for the follower ("Your party moves on — …")
}

// QueuePartySync appends a sync for this follower. Safe from any goroutine.
func (s *GameSession) QueuePartySync(p PartySync) {
	s.syncMu.Lock()
	s.partySyncs = append(s.partySyncs, p)
	s.syncMu.Unlock()
}

// TakePartySyncs removes and returns the queued syncs.
func (s *GameSession) TakePartySyncs() []PartySync {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	out := s.partySyncs
	s.partySyncs = nil
	return out
}

// PendingSave is an unsigned save event waiting for the player's signature.
type PendingSave struct {
	D         string // the event's d tag
	Content   string // compact save (savecodec) — the signed content must match exactly
	Tags      [][]string
	CreatedAt int64
	DiskJSON  []byte // the full save as it was when prepared, written to disk on success
}
