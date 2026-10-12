//go:build release

// Package pubkeyquest holds the game's assets for release builds: the one
// binary the official server runs carries its own templates, art, frontend
// bundle and game data (docs/draft/relay-saves-plan.md §6). Build with
// `go build -tags release ./cmd/server` after `npm run build`; release builds
// come from CI (see .github/workflows/release.yml), where only tracked files
// exist, so local-only folders never ship.
package pubkeyquest

import "embed"

// Embedded reports whether this binary carries its assets.
const Embedded = true

// Assets is the embedded asset tree, rooted at the repository root.
//
//go:embed www/views www/res www/dist www/manifest.json www/pwa.js www/sw.js all:game-data
var Assets embed.FS
