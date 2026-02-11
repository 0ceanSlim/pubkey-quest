// Package api provides HTTP API handlers for the Pubkey Quest server.
// This file contains route registration for all API endpoints.
//
// @title           Pubkey Quest API
// @version         1.0
// @description     REST API for the Pubkey Quest D&D-inspired RPG game server.
// @termsOfService  http://swagger.io/terms/
//
// @contact.name   Pubkey Quest Support
// @contact.url    https://github.com/0ceanSlim/pubkey-quest
//
// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT
//
// @BasePath  /api
//
// @securityDefinitions.apikey NostrAuth
// @in header
// @name X-Nostr-Pubkey
package api

import (
	"log"
	"net/http"

	"pubkey-quest/cmd/server/api/character"
	"pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/api/game"
	"pubkey-quest/cmd/server/auth"
	"pubkey-quest/cmd/server/utils"

	_ "pubkey-quest/docs/api/swagger"
	httpSwagger "github.com/swaggo/http-swagger"
)

// RegisterRoutes registers all API routes on the given ServeMux.
// This is the central location for all API endpoint registration.
//
// API Groups:
//   - /api/game-data, /api/items, /api/spells, etc. - Static game data
//   - /api/character, /api/weights, etc. - Character generation
//   - /api/auth/* - Authentication (Nostr)
//   - /api/saves/* - Save file management
//   - /api/session/* - In-memory session state
//   - /api/game/* - Game actions and state
//   - /api/shop/* - Shop transactions
//   - /api/profile - Player profiles
func RegisterRoutes(mux *http.ServeMux) {
	registerGameDataRoutes(mux)
	registerCharacterRoutes(mux)
	registerAuthRoutes(mux)
	registerSaveRoutes(mux)
	registerSessionRoutes(mux)
	registerGameRoutes(mux)
	registerShopRoutes(mux)
	registerProfileRoutes(mux)

	if utils.AppConfig.Server.DebugMode {
		registerDebugRoutes(mux)
	}

	mux.Handle("/api/docs/", httpSwagger.Handler(
		httpSwagger.URL("/api/docs/doc.json"),
	))
}

// ============================================================================
// Game Data Routes - Static game content (items, spells, monsters, etc.)
// ============================================================================

func registerGameDataRoutes(mux *http.ServeMux) {
	// @Summary Get all game data
	// @Description Returns all static game data in one request (items, spells, monsters, locations, packs, music)
	// @Tags GameData
	// @Produce json
	// @Success 200 {object} data.GameData
	// @Router /api/game-data [get]
	mux.HandleFunc("/api/game-data", data.GameDataHandler)

	// @Summary Get items
	// @Description Returns all items, optionally filtered by name
	// @Tags GameData
	// @Produce json
	// @Param name query string false "Filter by item ID"
	// @Success 200 {array} data.Item
	// @Router /api/items [get]
	mux.HandleFunc("/api/items", data.ItemsHandler)

	// @Summary Get spells
	// @Description Returns all spells or a specific spell by ID
	// @Tags GameData
	// @Produce json
	// @Param id path string false "Spell ID"
	// @Success 200 {array} data.Spell
	// @Router /api/spells/{id} [get]
	mux.HandleFunc("/api/spells/", data.SpellsHandler)

	// @Summary Get monsters
	// @Description Returns all monsters
	// @Tags GameData
	// @Produce json
	// @Success 200 {array} data.Monster
	// @Router /api/monsters [get]
	mux.HandleFunc("/api/monsters", data.MonstersHandler)

	// @Summary Get locations
	// @Description Returns all locations
	// @Tags GameData
	// @Produce json
	// @Success 200 {array} data.Location
	// @Router /api/locations [get]
	mux.HandleFunc("/api/locations", data.LocationsHandler)

	// @Summary Get NPCs
	// @Description Returns all NPCs
	// @Tags GameData
	// @Produce json
	// @Success 200 {array} data.NPC
	// @Router /api/npcs [get]
	mux.HandleFunc("/api/npcs", data.NPCsHandler)

	// @Summary Get NPCs at location
	// @Description Returns NPCs visible at player's current location and time
	// @Tags GameData
	// @Produce json
	// @Param location query string true "Location ID"
	// @Param district query string false "District ID"
	// @Param building query string false "Building ID"
	// @Param time query int false "Time of day in minutes"
	// @Success 200 {array} data.NPCLocationResponse
	// @Router /api/npcs/at-location [get]
	mux.HandleFunc("/api/npcs/at-location", data.GetNPCsAtLocationHandler)

	// @Summary Get abilities
	// @Description Returns abilities for martial classes (fighter, barbarian, monk, rogue)
	// @Tags GameData
	// @Produce json
	// @Param class query string true "Class name (fighter, barbarian, monk, rogue)"
	// @Param level query int false "Character level (default 1)"
	// @Success 200 {object} data.AbilitiesListResponse
	// @Router /api/abilities [get]
	mux.HandleFunc("/api/abilities", data.AbilitiesHandler)
}

// ============================================================================
// Character Routes - Character generation and creation
// ============================================================================

func registerCharacterRoutes(mux *http.ServeMux) {
	// @Summary Generate character
	// @Description Generates a deterministic character based on npub
	// @Tags Character
	// @Produce json
	// @Param npub query string true "Nostr public key (npub)"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/character [get]
	mux.HandleFunc("/api/character", character.CharacterHandler)

	// @Summary Create character save
	// @Description Creates a new character and save file
	// @Tags Character
	// @Accept json
	// @Produce json
	// @Param request body character.CreateCharacterRequest true "Character creation request"
	// @Success 200 {object} character.CreateCharacterResponse
	// @Router /api/character/create-save [post]
	mux.HandleFunc("/api/character/create-save", character.CreateCharacterHandler)

	// @Summary Get generation weights
	// @Description Returns character generation weight tables
	// @Tags Character
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /api/weights [get]
	mux.HandleFunc("/api/weights", character.WeightsHandler)

	// @Summary Get introductions
	// @Description Returns character introduction/backstory templates
	// @Tags Character
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /api/introductions [get]
	mux.HandleFunc("/api/introductions", character.IntroductionsHandler)

	// @Summary Get starting gear
	// @Description Returns starting equipment options by class
	// @Tags Character
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /api/starting-gear [get]
	mux.HandleFunc("/api/starting-gear", character.StartingGearHandler)
}

// ============================================================================
// Auth Routes - Nostr authentication
// ============================================================================

func registerAuthRoutes(mux *http.ServeMux) {
	authHandler := auth.NewAuthHandler(&utils.AppConfig)

	// @Summary Login
	// @Description Authenticate with Nostr
	// @Tags Auth
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /api/auth/login [post]
	mux.HandleFunc("/api/auth/login", authHandler.HandleLogin)

	// @Summary Logout
	// @Description End current session
	// @Tags Auth
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /api/auth/logout [post]
	mux.HandleFunc("/api/auth/logout", authHandler.HandleLogout)

	// @Summary Get session
	// @Description Check current authentication session
	// @Tags Auth
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /api/auth/session [get]
	mux.HandleFunc("/api/auth/session", authHandler.HandleSession)

	// @Summary Generate keys
	// @Description Generate a new Nostr keypair
	// @Tags Auth
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /api/auth/generate-keys [post]
	mux.HandleFunc("/api/auth/generate-keys", authHandler.HandleGenerateKeys)

	// @Summary Amber callback
	// @Description Handle Amber mobile auth callback
	// @Tags Auth
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /api/auth/amber-callback [get]
	mux.HandleFunc("/api/auth/amber-callback", authHandler.HandleAmberCallback)
}

// ============================================================================
// Save Routes - Save file management
// ============================================================================

func registerSaveRoutes(mux *http.ServeMux) {
	// @Summary Save operations
	// @Description GET: List saves, POST: Create/update save, DELETE: Delete save
	// @Tags Saves
	// @Produce json
	// @Param npub path string true "Nostr public key"
	// @Param saveID path string false "Save ID (for DELETE)"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/saves/{npub} [get]
	// @Router /api/saves/{npub} [post]
	// @Router /api/saves/{npub}/{saveID} [delete]
	mux.HandleFunc("/api/saves/", SavesHandler)
}

// ============================================================================
// Session Routes - In-memory game state management
// ============================================================================

func registerSessionRoutes(mux *http.ServeMux) {
	// @Summary Initialize session
	// @Description Load a save file into memory
	// @Tags Session
	// @Accept json
	// @Produce json
	// @Param request body object true "npub and save_id"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/session/init [post]
	mux.HandleFunc("/api/session/init", game.InitSessionHandler)

	// @Summary Reload session
	// @Description Force reload from disk, discarding in-memory changes
	// @Tags Session
	// @Accept json
	// @Produce json
	// @Param request body object true "npub and save_id"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/session/reload [post]
	mux.HandleFunc("/api/session/reload", game.ReloadSessionHandler)

	// @Summary Get session state
	// @Description Retrieve current in-memory session state
	// @Tags Session
	// @Produce json
	// @Param npub query string true "Nostr public key"
	// @Param save_id query string true "Save ID"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/session/state [get]
	mux.HandleFunc("/api/session/state", game.GetSessionHandler)

	// @Summary Update session
	// @Description Update in-memory game state
	// @Tags Session
	// @Accept json
	// @Produce json
	// @Param request body object true "npub, save_id, and save_data"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/session/update [post]
	mux.HandleFunc("/api/session/update", game.UpdateSessionHandler)

	// @Summary Save session
	// @Description Write in-memory state to disk
	// @Tags Session
	// @Accept json
	// @Produce json
	// @Param request body object true "npub and save_id"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/session/save [post]
	mux.HandleFunc("/api/session/save", game.SaveSessionHandler)

	// @Summary Cleanup session
	// @Description Remove session from memory
	// @Tags Session
	// @Produce json
	// @Param npub query string true "Nostr public key"
	// @Param save_id query string true "Save ID"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/session/cleanup [delete]
	mux.HandleFunc("/api/session/cleanup", game.CleanupSessionHandler)
}

// ============================================================================
// Game Routes - Game actions and state
// ============================================================================

func registerGameRoutes(mux *http.ServeMux) {
	// @Summary Game action
	// @Description Process a game action (move, use_item, equip, cast_spell, etc.)
	// @Tags Game
	// @Accept json
	// @Produce json
	// @Param request body object true "npub, save_id, and action"
	// @Success 200 {object} types.GameActionResponse
	// @Router /api/game/action [post]
	mux.HandleFunc("/api/game/action", game.GameActionHandler)

	// @Summary Get game state
	// @Description Returns current game state for a session
	// @Tags Game
	// @Produce json
	// @Param npub query string true "Nostr public key"
	// @Param save_id query string true "Save ID"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/game/state [get]
	mux.HandleFunc("/api/game/state", game.GetGameStateHandler)
}

// ============================================================================
// Shop Routes - Shop transactions
// ============================================================================

func registerShopRoutes(mux *http.ServeMux) {
	// @Summary Shop operations
	// @Description GET /{merchant_id}: Get shop data, POST /buy: Buy items, POST /sell: Sell items
	// @Tags Shop
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /api/shop/{merchant_id} [get]
	// @Router /api/shop/buy [post]
	// @Router /api/shop/sell [post]
	mux.HandleFunc("/api/shop/", game.ShopHandler)
}

// ============================================================================
// Profile Routes - Player profiles
// ============================================================================

func registerProfileRoutes(mux *http.ServeMux) {
	// @Summary Get profile
	// @Description Fetch Nostr profile metadata
	// @Tags Profile
	// @Produce json
	// @Param npub query string true "Nostr public key (npub)"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/profile [get]
	mux.HandleFunc("/api/profile", ProfileHandler)
}

// ============================================================================
// Debug Routes - Development/debugging endpoints (only in debug mode)
// ============================================================================

func registerDebugRoutes(mux *http.ServeMux) {
	log.Println("🐛 Debug mode enabled - registering debug routes")

	// @Summary List all sessions
	// @Description Returns all active sessions (debug only)
	// @Tags Debug
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /api/debug/sessions [get]
	mux.HandleFunc("/api/debug/sessions", func(w http.ResponseWriter, r *http.Request) {
		game.DebugSessionsHandler(w, r, true)
	})

	// @Summary Get session state
	// @Description Returns detailed session state (debug only)
	// @Tags Debug
	// @Produce json
	// @Param npub query string false "Nostr public key"
	// @Param save_id query string false "Save ID"
	// @Success 200 {object} map[string]interface{}
	// @Router /api/debug/state [get]
	mux.HandleFunc("/api/debug/state", func(w http.ResponseWriter, r *http.Request) {
		game.DebugStateHandler(w, r, true)
	})
}
