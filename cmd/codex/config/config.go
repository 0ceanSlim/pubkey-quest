package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents application configuration
type Config struct {
	Server struct {
		Port        int    `yaml:"port"`
		StagingMode string `yaml:"staging_mode"` // "auto", "direct", or "staging"
	} `yaml:"server"`
	GitHub struct {
		Token     string `yaml:"token"`
		RepoOwner string `yaml:"repo_owner"`
		RepoName  string `yaml:"repo_name"`
	} `yaml:"github"`
	PixelLab struct {
		APIKey string `yaml:"api_key"`
	} `yaml:"pixellab"`
	OpenAI struct {
		APIKey string `yaml:"api_key"` // scene and location art (--generate-images with provider "openai")
	} `yaml:"openai"`
}

// Load loads the application configuration.
// If configPath is empty, it defaults to ./codex-config.yml in the working directory.
func Load(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "./codex-config.yml"
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("⚠️ %s not found - using defaults", configPath)
		// Return default config
		config := &Config{}
		config.Server.Port = 8080
		config.Server.StagingMode = "auto"
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Set default port if not specified
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}

	// Set default staging mode if not specified
	if config.Server.StagingMode == "" {
		config.Server.StagingMode = "auto"
	}

	if config.PixelLab.APIKey == "" {
		log.Printf("⚠️ pixellab.api_key not found in config.yml - image generation will be disabled")
	}

	if config.GitHub.Token == "" {
		log.Printf("⚠️ github.token not found in config.yml - PR staging will be disabled")
	}

	return &config, nil
}
