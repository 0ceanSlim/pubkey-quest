package session

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"pubkey-quest/types"
)

// SavesDirectory is the path to save files
const SavesDirectory = "data/saves"

// init ensures the saves directory exists
func init() {
	if err := os.MkdirAll(SavesDirectory, 0755); err != nil {
		log.Printf("Warning: Failed to create saves directory: %v", err)
	}
}

// LoadSaveByID loads a specific save by npub and saveID
func LoadSaveByID(npub, saveID string) (*types.SaveFile, error) {
	savePath := filepath.Join(SavesDirectory, npub, saveID+".json")
	return LoadSaveFile(savePath)
}

// LoadSaveFile loads a save file from the given path
func LoadSaveFile(path string) (*types.SaveFile, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var save types.SaveFile
	if err := json.Unmarshal(data, &save); err != nil {
		return nil, err
	}

	// Extract internal ID from filename
	filename := filepath.Base(path)
	save.InternalID = strings.TrimSuffix(filename, ".json")

	// Extract npub from directory path
	dir := filepath.Dir(path)
	save.InternalNpub = filepath.Base(dir)

	// Schema migration: v1 saves unmarshal with zero-valued new fields (the
	// correct defaults); stamp them up to the current version. Future field
	// migrations key off the incoming SchemaVersion here.
	if save.SchemaVersion < types.CurrentSchemaVersion {
		save.SchemaVersion = types.CurrentSchemaVersion
	}

	return &save, nil
}

// WriteSaveFile writes a save file to disk
func WriteSaveFile(path string, save *types.SaveFile) error {
	data, err := json.MarshalIndent(save, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0644)
}

// GetSavePath returns the full path for a save file
func GetSavePath(npub, saveID string) string {
	return fmt.Sprintf("%s/%s/%s.json", SavesDirectory, npub, saveID)
}

// EnsureSaveDirectory ensures the saves directory exists for a user
func EnsureSaveDirectory(npub string) error {
	userSavesDir := filepath.Join(SavesDirectory, npub)
	return os.MkdirAll(userSavesDir, 0755)
}
