package session

import (
	"fmt"
	"log"
	"maps"
	"sync"
	"time"

	"pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/world"
	"pubkey-quest/types"
)

// ============================================================================
// Global Session Manager (with dependency injection)
// ============================================================================

// SessionManagerWrapper wraps SessionManager with app-specific dependencies
type SessionManagerWrapper struct {
	*SessionManager
}

// Global session manager instance
var globalSessionManager = &SessionManagerWrapper{
	SessionManager: NewSessionManager(),
}

// GetSessionManager returns the global session manager
func GetSessionManager() *SessionManagerWrapper {
	return globalSessionManager
}

// LoadSession loads a save file into memory with all dependencies injected
func (sm *SessionManagerWrapper) LoadSession(npub, saveID string) (*GameSession, error) {
	return sm.SessionManager.LoadSession(
		npub,
		saveID,
		loadAndHydrateSave,
		status.InitializeFatigueHungerEffects,
		data.GetNPCIDsAtLocation,
		getBuildingStatesWrapper,
	)
}

// ReloadSession forces a reload from disk with all dependencies injected. A
// reload is an explicit "discard in-memory changes", so it also drops any
// crash-recovery journal before reloading from the deliberate save.
func (sm *SessionManagerWrapper) ReloadSession(npub, saveID string) (*GameSession, error) {
	RemoveJournal(npub, saveID)
	return sm.SessionManager.ReloadSession(
		npub,
		saveID,
		loadAndHydrateSave,
		status.InitializeFatigueHungerEffects,
		data.GetNPCIDsAtLocation,
		getBuildingStatesWrapper,
	)
}

// getBuildingStatesWrapper wraps building.GetAllBuildingStatesForDistrict
func getBuildingStatesWrapper(location, district string, timeOfDay int) (map[string]bool, error) {
	database := db.GetDB()
	if database == nil {
		return nil, fmt.Errorf("database not available")
	}
	return building.GetAllBuildingStatesForDistrict(database, location, district, timeOfDay)
}

// loadAndHydrateSave loads a save from disk and recomputes its derived fields
// (MaxHP/MaxMana from class + level + stats) so leveling is reflected on every
// load. Advancement comes from the DB; if it's unavailable the raw save is
// returned and derived fields keep their last-persisted values.
func loadAndHydrateSave(npub, saveID string) (*types.SaveFile, error) {
	save, err := LoadSaveByID(npub, saveID)
	if err != nil {
		return nil, err
	}
	// Crash recovery: if a journal newer than the last deliberate save exists,
	// the server died with unsaved progress — restore that state instead.
	if recovered := RecoverJournaledSave(npub, saveID, GetSavePath(npub, saveID)); recovered != nil {
		log.Printf("📓 Recovered session %s:%s from journal (progress since last save)", npub, saveID)
		save = recovered
	}
	if database := db.GetDB(); database != nil {
		if adv, advErr := character.LoadAdvancement(database); advErr == nil {
			character.Hydrate(save, adv)
		} else {
			log.Printf("⚠️ Hydrate skipped — advancement load failed: %v", advErr)
		}
	}
	return save, nil
}

// ============================================================================
// Core Session Manager
// ============================================================================

// SessionManager manages all active game sessions in memory
type SessionManager struct {
	sessions map[string]*GameSession // Key: "{npub}:{saveID}"
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*GameSession),
	}
}

// sessionKey generates a key from npub and saveID
func sessionKey(npub, saveID string) string {
	return fmt.Sprintf("%s:%s", npub, saveID)
}

// LoadSession loads a save file into memory using the provided loader function
func (sm *SessionManager) LoadSession(npub, saveID string, loadSave func(string, string) (*types.SaveFile, error), initEffects func(*types.SaveFile) error, getNPCIDs func(string, string, string, string, int) []string, getBuildingStates func(string, string, int) (map[string]bool, error)) (*GameSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := sessionKey(npub, saveID)

	// Check if already loaded
	if session, exists := sm.sessions[key]; exists {
		return session, nil
	}

	// Load save file from disk
	saveData, err := loadSave(npub, saveID)
	if err != nil {
		return nil, fmt.Errorf("failed to load save file: %w", err)
	}

	// Initialize effects
	if initEffects != nil {
		if err := initEffects(saveData); err != nil {
			log.Printf("⚠️ Warning: Failed to initialize effects: %v", err)
		}
	}

	// Create new session in memory
	session := &GameSession{
		Npub:               npub,
		SaveID:             saveID,
		SaveData:           *saveData,
		LoadedAt:           currentTimestamp(),
		UpdatedAt:          currentTimestamp(),
		LastActionTime:     currentTimestamp(),
		LastActionGameTime: saveData.TimeOfDay,
		BuildingStates:     make(map[string]bool),
		Ground:             world.NewGroundStore(),
	}

	// Initialize building states and NPCs for current location
	timeOfDay := saveData.TimeOfDay
	currentHour := timeOfDay / 60

	// Load initial building states
	if getBuildingStates != nil {
		buildingStates, err := getBuildingStates(saveData.Location, saveData.District, timeOfDay)
		if err == nil && len(buildingStates) > 0 {
			session.BuildingStates = buildingStates
			session.BuildingsLastCheck = timeOfDay
		}
	}

	// Load initial NPCs at location
	if getNPCIDs != nil {
		npcIDs := getNPCIDs(saveData.Location, saveData.District, saveData.Building, saveData.Room, timeOfDay)
		session.NPCsAtLocation = npcIDs
		session.NPCsLastHour = currentHour
	}

	// Initialize snapshot for delta system
	session.InitializeSnapshot()

	sm.sessions[key] = session

	return session, nil
}

// ReloadSession forces a reload from disk, discarding in-memory changes
func (sm *SessionManager) ReloadSession(npub, saveID string, loadSave func(string, string) (*types.SaveFile, error), initEffects func(*types.SaveFile) error, getNPCIDs func(string, string, string, string, int) []string, getBuildingStates func(string, string, int) (map[string]bool, error)) (*GameSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := sessionKey(npub, saveID)

	// Load save file from disk (even if session exists in memory)
	saveData, err := loadSave(npub, saveID)
	if err != nil {
		return nil, fmt.Errorf("failed to load save file: %w", err)
	}

	// Initialize effects
	if initEffects != nil {
		if err := initEffects(saveData); err != nil {
			log.Printf("⚠️ Warning: Failed to initialize effects: %v", err)
		}
	}

	// Create/overwrite session in memory
	session := &GameSession{
		Npub:               npub,
		SaveID:             saveID,
		SaveData:           *saveData,
		LoadedAt:           currentTimestamp(),
		UpdatedAt:          currentTimestamp(),
		LastActionTime:     currentTimestamp(),
		LastActionGameTime: saveData.TimeOfDay,
		BuildingStates:     make(map[string]bool),
		Ground:             world.NewGroundStore(),
	}

	// Initialize building states and NPCs for current location
	timeOfDay := saveData.TimeOfDay
	currentHour := timeOfDay / 60

	// Load initial building states
	if getBuildingStates != nil {
		buildingStates, err := getBuildingStates(saveData.Location, saveData.District, timeOfDay)
		if err == nil && len(buildingStates) > 0 {
			session.BuildingStates = buildingStates
			session.BuildingsLastCheck = timeOfDay
		}
	}

	// Load initial NPCs at location
	if getNPCIDs != nil {
		npcIDs := getNPCIDs(saveData.Location, saveData.District, saveData.Building, saveData.Room, timeOfDay)
		session.NPCsAtLocation = npcIDs
		session.NPCsLastHour = currentHour
	}

	// Initialize snapshot for delta system
	session.InitializeSnapshot()

	sm.sessions[key] = session
	log.Printf("🔄 Session reloaded from disk (discarded in-memory changes): %s", key)

	return session, nil
}

// GetSession retrieves an active session from memory
func (sm *SessionManager) GetSession(npub, saveID string) (*GameSession, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key := sessionKey(npub, saveID)
	session, exists := sm.sessions[key]

	if !exists {
		return nil, fmt.Errorf("session not found in memory: %s", key)
	}

	return session, nil
}

// UpdateSession updates the in-memory game state
func (sm *SessionManager) UpdateSession(npub, saveID string, saveData types.SaveFile) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := sessionKey(npub, saveID)
	session, exists := sm.sessions[key]

	if !exists {
		return fmt.Errorf("session not found in memory: %s", key)
	}

	// Update the save data
	session.SaveData = saveData
	session.UpdatedAt = currentTimestamp()
	return nil
}

// SaveSessionToDisk is a placeholder - actual writing should be done by caller
func (sm *SessionManager) SaveSessionToDisk(npub, saveID string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key := sessionKey(npub, saveID)
	_, exists := sm.sessions[key]

	if !exists {
		return fmt.Errorf("session not found in memory: %s", key)
	}

	return nil // The actual write will happen in the handler
}

// UnloadSession removes a session from memory
func (sm *SessionManager) UnloadSession(npub, saveID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := sessionKey(npub, saveID)
	delete(sm.sessions, key)
}

// GetAllSessions returns all active sessions (for debugging)
func (sm *SessionManager) GetAllSessions() map[string]*GameSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Return a copy to avoid race conditions
	sessionsCopy := make(map[string]*GameSession, len(sm.sessions))
	maps.Copy(sessionsCopy, sm.sessions)

	return sessionsCopy
}

// currentTimestamp returns the current Unix timestamp
func currentTimestamp() int64 {
	return time.Now().Unix()
}

// InitializeSnapshot creates the initial snapshot for delta tracking
func (s *GameSession) InitializeSnapshot() {
	if s.BuildingStates == nil {
		s.BuildingStates = make(map[string]bool)
	}
	s.LastSnapshot = CreateSnapshot(&s.SaveData, s.NPCsAtLocation, s.BuildingStates)
	// Calculate initial show readiness
	s.LastSnapshot.ShowReady, s.LastSnapshot.ShowReadyBuilding = s.calculateShowReadiness()
}

// UpdateSnapshotAndCalculateDelta updates the snapshot and returns delta
func (s *GameSession) UpdateSnapshotAndCalculateDelta() *Delta {
	// Create new snapshot from current state
	newSnapshot := CreateSnapshot(&s.SaveData, s.NPCsAtLocation, s.BuildingStates)

	// Calculate show readiness based on booked shows and current time
	newSnapshot.ShowReady, newSnapshot.ShowReadyBuilding = s.calculateShowReadiness()

	// Log show_ready changes
	if s.LastSnapshot != nil && (s.LastSnapshot.ShowReady != newSnapshot.ShowReady || s.LastSnapshot.ShowReadyBuilding != newSnapshot.ShowReadyBuilding) {
		log.Printf("🎭 ShowReady changed: %v@%s -> %v@%s (time: %d, building: %s)",
			s.LastSnapshot.ShowReady, s.LastSnapshot.ShowReadyBuilding,
			newSnapshot.ShowReady, newSnapshot.ShowReadyBuilding,
			s.SaveData.TimeOfDay, s.SaveData.Building)
	}

	// Calculate delta from old to new
	var delta *Delta
	if s.LastSnapshot != nil {
		delta = CalculateDelta(s.LastSnapshot, newSnapshot)
	}

	// Update stored snapshot
	s.LastSnapshot = newSnapshot

	return delta
}

// UpdateSnapshotAndCalculateDeltaProvider returns a DeltaProvider interface
func (s *GameSession) UpdateSnapshotAndCalculateDeltaProvider() types.DeltaProvider {
	return s.UpdateSnapshotAndCalculateDelta()
}

// calculateShowReadiness checks if a booked show is ready to perform
func (s *GameSession) calculateShowReadiness() (bool, string) {
	if s.BookedShows == nil || len(s.BookedShows) == 0 {
		return false, ""
	}

	currentTime := s.SaveData.TimeOfDay
	currentDay := s.SaveData.CurrentDay

	for _, booking := range s.BookedShows {
		// Skip already performed shows
		performed, _ := booking["performed"].(bool)
		if performed {
			continue
		}

		bookingDay := 0
		if day, ok := booking["day"].(float64); ok {
			bookingDay = int(day)
		} else if day, ok := booking["day"].(int); ok {
			bookingDay = day
		}

		showTime := 1260 // Default 9 PM
		if st, ok := booking["show_time"].(float64); ok {
			showTime = int(st)
		} else if st, ok := booking["show_time"].(int); ok {
			showTime = st
		}

		venueID := ""
		if vid, ok := booking["venue_id"].(string); ok {
			venueID = vid
		}

		// Check if within the 60-minute show window (same day, show_time to show_time+60)
		if bookingDay == currentDay {
			timeDiff := currentTime - showTime
			if timeDiff >= 0 && timeDiff <= 60 {
				return true, venueID
			}
		}
	}

	return false, ""
}
