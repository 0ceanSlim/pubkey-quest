package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	pubkeyquest "pubkey-quest"
	"pubkey-quest/cmd/server/api"
	"pubkey-quest/cmd/server/app"
	"pubkey-quest/cmd/server/maintenance"
	"pubkey-quest/cmd/server/presence"
	"pubkey-quest/cmd/server/release"
	"pubkey-quest/cmd/server/routes"
	"pubkey-quest/cmd/server/utils"
)

// Version is set at build time via ldflags
var Version = "dev"

func main() {
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("pubkey-quest %s\n", Version)
		os.Exit(0)
	}

	log.Printf("🎮 Pubkey Quest %s", Version)
	utils.AppVersion = Version

	if err := utils.LoadConfig("config.yml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// A release binary carries its own assets: unpack them and build game.db
	// from the game data inside it before anything reads them.
	if pubkeyquest.Embedded {
		if err := release.Prepare(pubkeyquest.Assets, Version); err != nil {
			log.Fatalf("❌ Release start-up failed: %v", err)
		}
	}

	// The official server (validated saves) never runs debug tools.
	if utils.AppConfig.Saves.Validation && utils.AppConfig.Server.DebugMode {
		log.Fatalf("❌ config: saves.validation (the official server) can't run with server.debug_mode on")
	}

	app.Init()
	defer app.Shutdown()
	api.InitRelaySaves()

	// Deploys warn online players before a restart (data/restart-notice.json);
	// release builds also watch for a newer release to promote.
	maintenance.Start(func() int { return presence.OnlineCount(time.Now()) })
	if pubkeyquest.Embedded {
		maintenance.StartReleaseCheck(Version)
	}

	mux := http.NewServeMux()

	routes.RegisterStatic(mux)
	routes.RegisterPages(mux)
	routes.RegisterWiki(mux)
	api.RegisterRoutes(mux)

	app.Start(mux)
}
