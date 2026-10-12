// Package release unpacks a release binary's embedded assets and builds its
// game database, so the official server runs as one checksummed file
// (docs/draft/relay-saves-plan.md §6).
//
// The assets go to .release/<version>/ next to the working directory. The
// player data that must outlive any one version — data/ (saves, ledger,
// journals, reports) and config.yml — stays where it is and is linked in. The
// server then runs from the unpacked folder, so every relative path it uses
// today ("www/views", "game-data/…", "data/saves") keeps working unchanged.
package release

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"

	"pubkey-quest/cmd/codex/migration"
)

var safeVersion = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// Prepare unpacks assets for version, links the persistent data in, builds
// www/game.db from the embedded game data, and changes into the unpacked
// folder. Call it once at start-up, after loading config.
func Prepare(assets fs.FS, version string) error {
	home, err := os.Getwd()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".release", safeVersion.ReplaceAllString(version, "_"))
	marker := filepath.Join(dir, ".unpacked")
	if _, err := os.Stat(marker); err != nil {
		// Unpack fresh (a half-finished earlier attempt is discarded). Unlink
		// data first so the cleanup can never follow it into player saves.
		os.Remove(filepath.Join(dir, "data"))
		os.RemoveAll(dir)
		if err := unpack(assets, dir); err != nil {
			return fmt.Errorf("unpack assets: %w", err)
		}
		if err := os.WriteFile(marker, []byte(version+"\n"), 0o644); err != nil {
			return err
		}
		log.Printf("📦 Unpacked %s assets to %s", version, dir)
	}

	// Persistent data (saves, ledger, journals, reports) lives outside the
	// versioned folder and is linked in. config.yml was already read.
	if err := os.MkdirAll(filepath.Join(home, "data"), 0o755); err != nil {
		return err
	}
	if err := linkDir(filepath.Join(home, "data"), filepath.Join(dir, "data")); err != nil {
		return fmt.Errorf("link data: %w", err)
	}

	if err := os.Chdir(dir); err != nil {
		return err
	}
	// The database is a cache of the game data: rebuild it every start from
	// exactly the data this binary carries.
	os.Remove(filepath.Join("www", "game.db"))
	if err := migration.Migrate(filepath.Join("www", "game.db"), nil); err != nil {
		return fmt.Errorf("build game.db: %w", err)
	}
	log.Printf("✅ Release %s ready (game data migrated)", version)
	return nil
}

// unpack copies the embedded tree into dir.
func unpack(assets fs.FS, dir string) error {
	return fs.WalkDir(assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(assets, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// linkDir points link at target: a symlink, or on Windows (where symlinks
// need extra privileges) a directory junction.
func linkDir(target, link string) error {
	os.Remove(link)
	err := os.Symlink(target, link)
	if err == nil || runtime.GOOS != "windows" {
		return err
	}
	return exec.Command("cmd", "/c", "mklink", "/J", link, target).Run()
}
