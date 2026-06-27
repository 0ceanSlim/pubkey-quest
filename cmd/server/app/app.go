package app

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"pubkey-quest/cmd/server/auth"
	"pubkey-quest/cmd/server/cache"
	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/discovery"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/utils"
)

// Init initializes all application services
func Init() {
	if err := db.InitDatabase(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	if err := auth.InitializeGrainClient(&utils.AppConfig); err != nil {
		log.Fatalf("Failed to initialize Grain client: %v", err)
	}

	cache.InitProfileCache(24 * time.Hour)

	// Wire the event-recorder consumers: the quest objective tracker advances
	// active quests from gameplay events, and the discovery reward grants XP for
	// reaching new places. Both need the advancement table for level-ups.
	if adv, err := character.LoadAdvancement(db.GetDB()); err != nil {
		log.Printf("⚠️ event consumers: failed to load advancement: %v", err)
	} else {
		events.Subscribe(quest.Consumer(db.GetQuestByID, adv))
		events.Subscribe(discovery.XPConsumer(adv))
		log.Println("✅ Quest tracker + discovery rewards registered")
	}

	// Crash resilience: periodically snapshot active sessions so an unexpected
	// server death doesn't eat unsaved progress (restored on next load).
	session.StartJournalLoop(2 * time.Minute)

	log.Println("✅ All services initialized")
}

// Shutdown cleans up all application services
func Shutdown() {
	// Snapshot active sessions so a clean restart can recover in-progress play.
	session.JournalAllSessions()
	db.Close()
	auth.ShutdownGrainClient()
	log.Println("✅ All services shut down")
}

// Start starts the HTTP server
func Start(mux *http.ServeMux) {
	port := utils.AppConfig.Server.Port
	fmt.Printf("Server running on http://localhost:%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
