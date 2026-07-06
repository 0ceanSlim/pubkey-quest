package auth

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/session"
	cfgType "github.com/0ceanslim/grain/config/types"
	"pubkey-quest/cmd/server/utils"
)

// grainCancel cancels the background goroutines (session/cache cleanup, relay
// health check, outbox eviction sweeper) started by InitializeGrainClient. It
// is invoked by ShutdownGrainClient.
var grainCancel context.CancelFunc

// InitializeGrainClient initializes the grain client for Pubkey Quest.
//
// We deliberately do NOT import the top-level github.com/0ceanslim/grain/client
// package: in grain 0.8 that package compiles the relay/admin web UI, which
// pulls in the cgo-backed server/db/nostrdb store (needs nostrdb.h). Pubkey
// Quest only needs the outbox-capable client core + session manager, so we wire
// those subpackages directly — mirroring what client.InitializeClient does,
// minus the server stack.
func InitializeGrainClient(config *utils.Config) error {
	log.Println("🎮 Initializing Grain client for Pubkey Quest...")

	// Minimal server config for the client core; ConfigFromServerConfig fills
	// in sane defaults for anything left zero and validates the result.
	grainConfig := &cfgType.ServerConfig{
		Server: cfgType.ServerSettings{
			Port:                      fmt.Sprintf("%d", config.Server.Port),
			ReadTimeout:               30,
			WriteTimeout:              30,
			IdleTimeout:               60,
			MaxSubscriptionsPerClient: 10,
			ImplicitReqLimit:          10,
		},
	}

	// Session manager (holds authenticated pubkey sessions).
	session.SessionMgr = session.NewSessionManager()
	if session.SessionMgr == nil {
		log.Printf("❌ Failed to create session manager")
		return fmt.Errorf("failed to create session manager")
	}

	// Outbox-capable core client (NIP-65 relay routing / mailbox resolution).
	if err := connection.InitializeCoreClient(grainConfig); err != nil {
		log.Printf("❌ Failed to initialize Grain core client: %v", err)
		return fmt.Errorf("failed to initialize grain core client: %w", err)
	}

	// Indexer/seed relays used to resolve relay lists + profile metadata for
	// arbitrary users (grain's built-in indexer seed set).
	connection.SetIndexRelays([]string{
		"wss://profiles.nostr1.com",
		"wss://directory.yabu.me",
		"wss://user.kindpag.es",
		"wss://indexer.coracle.social",
		"wss://purplepag.es",
	})

	// Background maintenance goroutines, bounded by grainCancel.
	ctx, cancel := context.WithCancel(context.Background())
	grainCancel = cancel
	cache.StartCacheCleanup(ctx)
	connection.StartRelayHealthCheck(ctx, 5*time.Minute)
	connection.StartRelayEvictionSweeper(ctx, time.Minute)

	log.Println("✅ Grain client ready for Pubkey Quest")
	return nil
}

// ShutdownGrainClient gracefully shuts down the grain client.
func ShutdownGrainClient() error {
	log.Println("🎮 Shutting down Grain client...")
	if grainCancel != nil {
		grainCancel()
	}
	if err := connection.CloseCoreClient(); err != nil {
		log.Printf("⚠️ Error closing grain core client: %v", err)
		return err
	}
	session.SessionMgr = nil
	return nil
}
