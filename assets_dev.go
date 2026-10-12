//go:build !release

// Package pubkeyquest holds the game's assets for release builds; dev builds
// read everything from the working tree instead (see assets_release.go).
package pubkeyquest

import "embed"

// Embedded reports whether this binary carries its assets.
const Embedded = false

// Assets is empty in dev builds.
var Assets embed.FS
