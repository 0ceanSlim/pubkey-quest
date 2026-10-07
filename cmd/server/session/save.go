package session

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"slices"
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
	// correct defaults); stamp them up to the current version. Field migrations
	// key off the incoming SchemaVersion here.
	if save.SchemaVersion < 4 {
		migrateVaultsToShared(&save)
	}
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

// migrateVaultsToShared folds the pre-v4 per-building vault grids into the one
// shared vault (schema v4). Every grid's contents pour into the same pool —
// nothing is lost, and duplicates across towns merge into a single stack, since
// the shared vault ignores stack limits. Each grid's building ID becomes a
// keeper registration, preserving who had already accepted the player.
//
// Containers deposited under the old rules could hold items; v4 requires vault
// containers to be empty, so a container's contents are poured into the pool
// alongside it rather than silently dropped.
func migrateVaultsToShared(save *types.SaveFile) {
	if len(save.LegacyVaults) == 0 {
		save.LegacyVaults = nil
		return
	}

	pool := map[string]int{}
	var order []string
	add := func(itemID string, qty int) {
		if itemID == "" || qty <= 0 {
			return
		}
		if _, seen := pool[itemID]; !seen {
			order = append(order, itemID)
		}
		pool[itemID] += qty
	}

	for _, old := range save.LegacyVaults {
		if building, ok := old["building"].(string); ok && building != "" {
			if !slices.Contains(save.VaultKeepers, building) {
				save.VaultKeepers = append(save.VaultKeepers, building)
			}
		}
		slots, ok := old["slots"].([]interface{})
		if !ok {
			continue
		}
		for _, raw := range slots {
			slot, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			itemID, _ := slot["item"].(string)
			add(itemID, legacyQty(slot["quantity"]))
			// A container's own contents ride along as loose stacks.
			nested, ok := slot["contents"].([]interface{})
			if !ok {
				continue
			}
			for _, rawInner := range nested {
				inner, ok := rawInner.(map[string]interface{})
				if !ok {
					continue
				}
				innerID, _ := inner["item"].(string)
				add(innerID, legacyQty(inner["quantity"]))
			}
		}
	}

	for _, itemID := range order {
		save.Vault = append(save.Vault, types.VaultEntry{ItemID: itemID, Quantity: pool[itemID]})
	}
	save.LegacyVaults = nil

	log.Printf("🔄 Vault migration: %d legacy vault(s) merged into %d shared entr(ies), %d keeper(s) registered",
		len(order), len(save.Vault), len(save.VaultKeepers))
}

// legacyQty reads a quantity that may have been stored as either a JSON number
// or an int, defaulting to 1 for a filled slot that never recorded one.
func legacyQty(raw interface{}) int {
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 1
}
