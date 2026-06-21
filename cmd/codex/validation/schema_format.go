// Schema-draft pretty-printer.
//
// Re-formats POI / Encounter / Quest draft JSON files with consistent
// 2-space indentation. json.Indent preserves insertion order because it
// reformats the existing bytes rather than re-marshaling, so author key
// order is kept intact.
//
// Lifted from cmd/schemafmt (now removed) so Codex is the single
// formatting surface for game data.
package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// FormatSchemaBytes returns the input JSON re-indented with 2 spaces and
// a trailing newline. Returns an error only if the input is not valid JSON.
func FormatSchemaBytes(in []byte) ([]byte, error) {
	var out bytes.Buffer
	if err := json.Indent(&out, in, "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// FormatSchemaFile reads, formats, and writes back a single JSON file.
func FormatSchemaFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := FormatSchemaBytes(b)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// FormatSchemaResult summarises a directory-wide formatting run.
type FormatSchemaResult struct {
	Formatted []string `json:"formatted"`
	Failures  []string `json:"failures"`
}

// FormatSchemaDirs formats every .json file under each given directory.
// If dirs is empty, defaults to the canonical draft directories.
func FormatSchemaDirs(dirs ...string) (*FormatSchemaResult, error) {
	if len(dirs) == 0 {
		d := DefaultSchemaDirs()
		dirs = []string{d.POIs, d.Encounters, d.Quests}
	}
	res := &FormatSchemaResult{}
	for _, d := range dirs {
		for _, p := range schemaWalkJSON(d) {
			if err := FormatSchemaFile(p); err != nil {
				res.Failures = append(res.Failures, fmt.Sprintf("FAIL %s: %v", p, err))
				continue
			}
			res.Formatted = append(res.Formatted, p)
		}
	}
	return res, nil
}
