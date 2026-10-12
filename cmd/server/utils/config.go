package utils

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds server-related configurations
type ServerConfig struct {
	Port      int      `yaml:"port"`
	TLS       bool     `yaml:"tls"`
	AppTitle  string   `yaml:"app_title"`
	DebugMode bool     `yaml:"debug_mode"`
	Whitelist []string `yaml:"whitelist"` // Pubkey whitelist (npub or hex format) - enforced when debug_mode is true
}

// ReportConfig configures the in-game reporter (bug reports + access requests).
// Every submission is always appended to a local JSONL log under data/reports/;
// when a GitHub token and the matching issue number are set, the submission is
// also posted as a comment on that pinned collective issue (which is how the
// maintainer gets notified by email). Leaving GitHubToken or an issue number
// empty/zero disables the GitHub post for that report type and logs only.
type ReportConfig struct {
	GitHubToken       string `yaml:"github_token"`        // Server-side PAT with repo:issues scope (config.yml is gitignored)
	GitHubRepo        string `yaml:"github_repo"`         // "owner/repo", e.g. "0ceanSlim/pubkey-quest"
	BugIssueNumber    int    `yaml:"bug_issue_number"`    // Pinned issue that collects bug reports
	AccessIssueNumber int    `yaml:"access_issue_number"` // Pinned issue that collects access requests
}

// Config holds the full application configuration
// SavesConfig decides how saves work on this server (docs/draft/relay-saves-plan.md §5).
//
//   - Relay: saving signs a kind-30078 event in the browser and publishes it to
//     the player's relays; loading prefers the relay copy. Off = disk saves only.
//   - Validation: the official server's rule — only the save whose event id this
//     server last accepted is playable. Off on test / modded servers.
//   - MaxSlots: characters per key (official: 5). 0 = unlimited.
type SavesConfig struct {
	Relay      bool `yaml:"relay"`
	Validation bool `yaml:"validation"`
	MaxSlots   int  `yaml:"max_slots"`
}

type Config struct {
	Server ServerConfig `yaml:"server"`
	Report ReportConfig `yaml:"report"`
	Saves  SavesConfig  `yaml:"saves"`
}

// Global variable to hold the config after loading
var AppConfig Config

// LoadConfig reads the YAML config file into the AppConfig struct
func LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	err = yaml.Unmarshal(data, &AppConfig)
	if err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return nil
}
