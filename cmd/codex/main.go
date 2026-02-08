package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"pubkey-quest/cmd/codex/charactereditor"
	"pubkey-quest/cmd/codex/config"
	"pubkey-quest/cmd/codex/itemeditor"
	"pubkey-quest/cmd/codex/migration"
	"pubkey-quest/cmd/codex/pixellab"
	"pubkey-quest/cmd/codex/staging"
	"pubkey-quest/cmd/codex/systemseditor"
	"pubkey-quest/cmd/codex/validation"

	"github.com/gorilla/mux"
)

var editor *itemeditor.Editor
var charEditor *charactereditor.Editor
var sysEditor *systemseditor.Editor
var cfg *config.Config

// Version is set at build time via ldflags
var Version = "dev"

func main() {
	// Command-line flags
	migrateFlag := flag.Bool("migrate", false, "Run database migration and exit")
	validateFlag := flag.Bool("validate", false, "Run game data validation and exit")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	configFlag := flag.String("config", "", "Path to config file (default: ./codex-config.yml)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("codex %s\n", Version)
		os.Exit(0)
	}

	// Handle validate flag (no config required)
	if *validateFlag {
		fmt.Println("🔍 Running game data validation...")
		result, err := validation.ValidateAll()
		if err != nil {
			fmt.Printf("❌ Validation failed to run: %v\n", err)
			os.Exit(1)
		}

		// Print results
		fmt.Printf("\n📊 Validation Results:\n")
		fmt.Printf("   Files scanned: %d\n", result.Stats.TotalFiles)
		fmt.Printf("   Errors: %d\n", result.Stats.ErrorCount)
		fmt.Printf("   Warnings: %d\n", result.Stats.WarningCount)

		if len(result.Issues) > 0 {
			fmt.Printf("\n📋 Issues:\n")
			for _, issue := range result.Issues {
				prefix := "⚠️"
				if issue.Type == "error" {
					prefix = "❌"
				} else if issue.Type == "info" {
					prefix = "ℹ️"
				}
				fmt.Printf("   %s [%s] %s: %s\n", prefix, issue.Category, issue.File, issue.Message)
			}
		}

		if result.Stats.ErrorCount > 0 {
			fmt.Printf("\n❌ Validation failed with %d error(s)\n", result.Stats.ErrorCount)
			os.Exit(1)
		}

		fmt.Println("\n✅ Validation passed!")
		os.Exit(0)
	}

	// Handle migration flag (no config required)
	if *migrateFlag {
		fmt.Println("🔄 Running database migration...")
		dbPath := "./www/game.db"

		err := migration.Migrate(dbPath, func(status migration.Status) {
			if status.Progress > 0 {
				fmt.Printf("  %s (%d/%d)\n", status.Message, status.Progress, status.Total)
			} else {
				fmt.Printf("  %s\n", status.Message)
			}
		})

		if err != nil {
			fmt.Printf("❌ Migration failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Migration completed successfully!")
		os.Exit(0)
	}

	// Load configuration (required for web server)
	var err error
	cfg, err = config.Load(*configFlag)
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}
	if cfg == nil {
		log.Fatal("❌ codex-config.yml not found")
	}

	// Initialize item editor
	editor = itemeditor.New()
	editor.Config = cfg

	if err := editor.LoadItems(); err != nil {
		log.Fatal(err)
	}

	// Initialize PixelLab if API key is configured
	if cfg.PixelLab.APIKey != "" {
		editor.PixelLabClient = pixellab.NewClient(cfg.PixelLab.APIKey)
		log.Printf("✅ PixelLab client initialized")
	}

	// Initialize character editor
	charEditor = charactereditor.NewEditor(cfg)
	if err := charEditor.LoadAll(); err != nil {
		log.Printf("⚠️ Warning: Failed to load character data: %v", err)
	} else {
		log.Printf("✅ Character editor initialized")
	}

	// Initialize systems editor
	sysEditor = systemseditor.NewEditor(cfg)
	if err := sysEditor.LoadAll(); err != nil {
		log.Printf("⚠️ Warning: Failed to load systems data: %v", err)
	} else {
		log.Printf("✅ Systems editor initialized with %d effects", len(sysEditor.Effects))
	}

	// Initialize staging system
	staging.SetConfig(cfg)
	staging.Manager.StartCleanupRoutine()
	log.Printf("✅ Staging system initialized")

	r := mux.NewRouter()

	// Home page
	r.HandleFunc("/", handleHome).Methods("GET")

	// Starting Gear editor routes
	r.HandleFunc("/tools/starting-gear-editor", handleStartingGearEditor).Methods("GET")
	r.HandleFunc("/api/character-data", charEditor.HandleGetAllData).Methods("GET")
	r.HandleFunc("/api/character-data/starting-gear", charEditor.HandleGetStartingGear).Methods("GET")
	r.HandleFunc("/api/character-data/starting-gear", charEditor.HandleSaveStartingGear).Methods("PUT")
	r.HandleFunc("/api/character-data/base-hp", charEditor.HandleGetOtherFile("base-hp")).Methods("GET")
	r.HandleFunc("/api/character-data/base-hp", charEditor.HandleSaveOtherFile("base-hp")).Methods("PUT")
	r.HandleFunc("/api/character-data/starting-gold", charEditor.HandleGetOtherFile("starting-gold")).Methods("GET")
	r.HandleFunc("/api/character-data/starting-gold", charEditor.HandleSaveOtherFile("starting-gold")).Methods("PUT")
	r.HandleFunc("/api/character-data/generation-weights", charEditor.HandleGetOtherFile("generation-weights")).Methods("GET")
	r.HandleFunc("/api/character-data/generation-weights", charEditor.HandleSaveOtherFile("generation-weights")).Methods("PUT")
	r.HandleFunc("/api/character-data/introductions", charEditor.HandleGetOtherFile("introductions")).Methods("GET")
	r.HandleFunc("/api/character-data/introductions", charEditor.HandleSaveOtherFile("introductions")).Methods("PUT")
	r.HandleFunc("/api/character-data/starting-locations", charEditor.HandleGetOtherFile("starting-locations")).Methods("GET")
	r.HandleFunc("/api/character-data/starting-locations", charEditor.HandleSaveOtherFile("starting-locations")).Methods("PUT")
	r.HandleFunc("/api/character-data/starting-spells", charEditor.HandleGetOtherFile("starting-spells")).Methods("GET")
	r.HandleFunc("/api/character-data/starting-spells", charEditor.HandleSaveOtherFile("starting-spells")).Methods("PUT")

	// Systems editor routes
	r.HandleFunc("/tools/systems-editor", sysEditor.HandlePage).Methods("GET")
	r.HandleFunc("/api/systems-data", sysEditor.HandleGetSystemsData).Methods("GET")
	r.HandleFunc("/api/effects", sysEditor.HandleGetEffects).Methods("GET")
	r.HandleFunc("/api/effects", sysEditor.HandleCreateEffect).Methods("POST")
	r.HandleFunc("/api/effects/{id}", sysEditor.HandleSaveEffect).Methods("PUT")
	r.HandleFunc("/api/effects/{id}", sysEditor.HandleDeleteEffect).Methods("DELETE")
	r.HandleFunc("/api/effect-types", sysEditor.HandleGetEffectTypes).Methods("GET")
	r.HandleFunc("/api/effect-types", sysEditor.HandleSaveEffectTypes).Methods("PUT")

	// Item editor routes
	r.HandleFunc("/tools/item-editor", editor.HandleItemEditor).Methods("GET")
	r.HandleFunc("/api/items", editor.HandleGetItems).Methods("GET")
	r.HandleFunc("/api/items/{filename}", editor.HandleGetItem).Methods("GET")
	r.HandleFunc("/api/items/{filename}", editor.HandleSaveItem).Methods("PUT")
	r.HandleFunc("/api/items/{filename}", editor.HandleDeleteItem).Methods("DELETE")
	r.HandleFunc("/api/validate", editor.HandleValidate).Methods("GET")
	r.HandleFunc("/api/types", editor.HandleGetTypes).Methods("GET")
	r.HandleFunc("/api/tags", editor.HandleGetTags).Methods("GET")
	r.HandleFunc("/api/refactor/preview", editor.HandleRefactorPreview).Methods("POST")
	r.HandleFunc("/api/refactor/apply", editor.HandleRefactorApply).Methods("POST")
	r.HandleFunc("/api/balance", editor.HandleGetBalance).Methods("GET")
	r.HandleFunc("/api/items/{filename}/generate-image", editor.HandleGenerateImage).Methods("POST")
	r.HandleFunc("/api/items/{filename}/image", editor.HandleGetImage).Methods("GET")
	r.HandleFunc("/api/items/{filename}/accept-image", editor.HandleAcceptImage).Methods("POST")

	// Database migration routes
	r.HandleFunc("/tools/database-migration", handleDatabaseMigration).Methods("GET")
	r.HandleFunc("/api/migrate/start", handleMigrateStart).Methods("POST")
	r.HandleFunc("/api/migrate/status", handleMigrateStatus).Methods("GET")

	// Validation routes
	r.HandleFunc("/tools/validation", handleValidationTool).Methods("GET")
	r.HandleFunc("/api/validation/run", handleValidationRun).Methods("POST")
	r.HandleFunc("/api/validation/cleanup", handleCleanupRun).Methods("POST")
	r.HandleFunc("/api/validation/item/{itemId}", handleValidateOneItem).Methods("GET")

	// Staging routes
	r.HandleFunc("/api/staging/init", staging.HandleStagingInit).Methods("POST")
	r.HandleFunc("/api/staging/changes", staging.HandleGetStagingChanges).Methods("GET")
	r.HandleFunc("/api/staging/submit", staging.HandleStagingSubmit).Methods("POST")
	r.HandleFunc("/api/staging/clear", staging.HandleStagingClear).Methods("DELETE")
	r.HandleFunc("/api/staging/mode", staging.HandleGetMode).Methods("GET")

	// Static files - serve from main www directory (running from root)
	r.PathPrefix("/www/").Handler(http.StripPrefix("/www/", http.FileServer(http.Dir("www"))))
	r.PathPrefix("/game-res/").Handler(http.StripPrefix("/game-res/", http.FileServer(http.Dir("www/res"))))

	// CODEX built assets - served from www/dist/codex/
	r.PathPrefix("/dist/codex/").Handler(http.StripPrefix("/dist/codex/", http.FileServer(http.Dir("www/dist/codex"))))

	// CODEX resources (if any static files in res/)
	r.PathPrefix("/codex-res/").Handler(http.StripPrefix("/codex-res/", http.FileServer(http.Dir("cmd/codex/res"))))

	port := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Println("🎯 CODEX - Content Organization & Data Entry eXperience")
	fmt.Printf("🚀 Server starting on http://localhost%s\n", port)

	log.Fatal(http.ListenAndServe(port, r))
}

// HomeData holds data for the home page template
type HomeData struct {
	IsStaging bool
	Version   string
}

// Home page handler
func handleHome(w http.ResponseWriter, r *http.Request) {
	mode := staging.DetectMode(r, cfg)

	tmpl, err := template.ParseFiles("cmd/codex/html/home.html")
	if err != nil {
		http.Error(w, "Failed to load template", http.StatusInternalServerError)
		log.Printf("❌ Template error: %v", err)
		return
	}

	data := HomeData{
		IsStaging: mode == staging.ModeStaging,
		Version:   Version,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("❌ Template execution error: %v", err)
	}
}

// Database migration handlers
var migrationStatus migration.Status
var migrationRunning bool

func handleDatabaseMigration(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "cmd/codex/html/database-migration.html")
}

func handleMigrateStart(w http.ResponseWriter, r *http.Request) {
	if migrationRunning {
		http.Error(w, "Migration already in progress", http.StatusConflict)
		return
	}

	migrationRunning = true
	go func() {
		defer func() { migrationRunning = false }()

		dbPath := "./www/game.db"
		err := migration.Migrate(dbPath, func(status migration.Status) {
			migrationStatus = status
		})

		if err != nil {
			migrationStatus = migration.Status{
				Step:  "error",
				Error: err.Error(),
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func handleMigrateStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(migrationStatus)
}

// Validation handlers
func handleValidationTool(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "cmd/codex/html/validation.html")
}

func handleValidationRun(w http.ResponseWriter, r *http.Request) {
	result, err := validation.ValidateAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleValidateOneItem(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["itemId"]

	issues, err := validation.ValidateOneItem(itemID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Calculate stats
	errorCount := 0
	warningCount := 0
	for _, issue := range issues {
		if issue.Type == "error" {
			errorCount++
		} else if issue.Type == "warning" {
			warningCount++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"issues":        issues,
		"error_count":   errorCount,
		"warning_count": warningCount,
		"valid":         errorCount == 0,
	})
}

func handleCleanupRun(w http.ResponseWriter, r *http.Request) {
	// Check for dry_run parameter
	dryRun := r.URL.Query().Get("dry_run") == "true"

	result, err := validation.CleanupAllItems(dryRun)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Starting Gear editor handler
func handleStartingGearEditor(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "cmd/codex/html/starting-gear-editor.html")
}
