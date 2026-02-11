package auth

import (
	"fmt"
	"log"

	"github.com/0ceanslim/grain/client"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/client/session"
	"pubkey-quest/cmd/server/utils"
)

// InitializeGrainClient initializes grain client for Pubkey Quest
func InitializeGrainClient(config *utils.Config) error {
	log.Println("🎮 Initializing Grain client for Pubkey Quest...")

	// Create minimal grain server config for session management only
	grainConfig := &cfgType.ServerConfig{
		Server: struct {
			Port                      string `yaml:"port"`
			ReadTimeout               int    `yaml:"read_timeout"`
			WriteTimeout              int    `yaml:"write_timeout"`
			IdleTimeout               int    `yaml:"idle_timeout"`
			MaxSubscriptionsPerClient int    `yaml:"max_subscriptions_per_client"`
			ImplicitReqLimit          int    `yaml:"implicit_req_limit"`
		}{
			Port:                      fmt.Sprintf("%d", config.Server.Port),
			ReadTimeout:               30,
			WriteTimeout:              30,
			IdleTimeout:               60,
			MaxSubscriptionsPerClient: 10,
			ImplicitReqLimit:          10,
		},
		// Initialize other required fields with defaults
		Client: cfgType.ClientConfig{
			// Default client config
		},
		RateLimit: cfgType.RateLimitConfig{
			// Default rate limit config
		},
		Blacklist: cfgType.BlacklistConfig{
			// Default blacklist config
		},
		ResourceLimits: cfgType.ResourceLimits{
			// Default resource limits
		},
		Auth: cfgType.AuthConfig{
			// Default auth config
		},
		EventPurge: cfgType.EventPurgeConfig{
			// Default event purge config
		},
		EventTimeConstraints: cfgType.EventTimeConstraints{
			// Default event time constraints
		},
	}

	// Initialize the grain client
	if err := client.InitializeClient(grainConfig); err != nil {
		log.Printf("❌ Failed to initialize Grain client: %v", err)
		return fmt.Errorf("failed to initialize grain client: %w", err)
	}

	// Initialize the session manager
	session.SessionMgr = session.NewSessionManager()
	if session.SessionMgr == nil {
		log.Printf("❌ Failed to create session manager")
		return fmt.Errorf("failed to create session manager")
	}

	log.Println("✅ Grain client ready for Pubkey Quest")
	return nil
}

// ShutdownGrainClient gracefully shuts down the grain client
func ShutdownGrainClient() error {
	log.Println("🎮 Shutting down Grain client...")
	return nil
}

// getDefaultRelays returns default Nostr relays for the game
func getDefaultRelays(config *utils.Config) []string {
	// You can configure these in your config file later
	defaultRelays := []string{
		"wss://relay.damus.io",
		"wss://nos.lol",
		"wss://relay.nostr.band",
		"wss://nostr.happytavern.co",
		"wss://relay.snort.social",
	}

	// TODO: Add config option to override default relays
	// if config.Nostr.DefaultRelays != nil {
	//     return config.Nostr.DefaultRelays
	// }

	return defaultRelays
}