package itemeditor

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"pubkey-quest/cmd/codex/pixellab"
)

// Item represents a game item with all possible fields
type Item struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Description    string              `json:"description,omitempty"`
	Rarity         string              `json:"rarity"`
	Price          int                 `json:"price"`
	Weight         float64             `json:"weight"`
	Stack          int                 `json:"stack"`
	Type           string              `json:"type"`
	GearSlot       string              `json:"gear_slot,omitempty"`
	ContainerSlots int                 `json:"container_slots,omitempty"`
	AllowedTypes   interface{}         `json:"allowed_types,omitempty"`
	AC             interface{}         `json:"ac,omitempty"`
	Damage         interface{}         `json:"damage,omitempty"`
	DamageType     string              `json:"damage-type,omitempty"`
	Heal           interface{}         `json:"heal,omitempty"`
	Ammunition     string              `json:"ammunition,omitempty"`
	Range          string              `json:"range,omitempty"`
	RangeLong      string              `json:"range-long,omitempty"`
	Effects         []interface{}          `json:"effects,omitempty"`
	EffectsWhenWorn []string               `json:"effects_when_worn,omitempty"`
	Tags            []string               `json:"tags,omitempty"`
	Notes          []string            `json:"notes,omitempty"`
	Image          string              `json:"image,omitempty"`
	Img            string              `json:"img,omitempty"`
	Contents       [][]interface{}     `json:"contents,omitempty"`
	Provides       string              `json:"provides,omitempty"`
	Extra          map[string]interface{} `json:"-"`
}

// Editor holds the item editor state
type Editor struct {
	Items          map[string]*Item
	PixelLabClient *pixellab.Client
	Config         interface{} // Will be *config.Config, using interface to avoid import cycle
}

// New creates a new item editor instance
func New() *Editor {
	return &Editor{
		Items: make(map[string]*Item),
	}
}

// LoadItems loads all items from the items directory
func (e *Editor) LoadItems() error {
	itemsDir := "game-data/items"
	e.Items = make(map[string]*Item)

	err := filepath.WalkDir(itemsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			var item Item
			if err := json.Unmarshal(data, &item); err != nil {
				return fmt.Errorf("error parsing %s: %v", path, err)
			}

			filename := strings.TrimSuffix(filepath.Base(path), ".json")
			e.Items[filename] = &item
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Printf("✅ Loaded %d items from %s", len(e.Items), itemsDir)
	return nil
}

// SaveItemToFile writes an item to its JSON file
func (e *Editor) SaveItemToFile(filename string, item *Item) error {
	itemsDir := "game-data/items"
	filepath := filepath.Join(itemsDir, filename+".json")

	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}
