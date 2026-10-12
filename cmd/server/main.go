package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"pubkey-quest/cmd/server/api"
	"pubkey-quest/cmd/server/app"
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

	// The official server (validated saves) never runs debug tools.
	if utils.AppConfig.Saves.Validation && utils.AppConfig.Server.DebugMode {
		log.Fatalf("❌ config: saves.validation (the official server) can't run with server.debug_mode on")
	}

	app.Init()
	defer app.Shutdown()
	api.InitRelaySaves()

	mux := http.NewServeMux()

	routes.RegisterStatic(mux)
	routes.RegisterPages(mux)
	api.RegisterRoutes(mux)

	app.Start(mux)
}
