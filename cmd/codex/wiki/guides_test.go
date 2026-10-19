package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteGuides renders the Rules guides from the real game data and checks
// each page exists and carries no formatting artefacts.
func TestWriteGuides(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data := filepath.Join(root, "game-data")
	if _, err := os.Stat(data); err != nil {
		t.Skip("game-data not found")
	}
	out := t.TempDir()
	g := &gen{opt: Options{DataDir: data, ImageDir: filepath.Join(root, "www", "res", "img"), OutDir: out, BasePath: "/wiki"},
		written: map[string]bool{}, res: &Result{}}
	if err := g.load(); err != nil {
		t.Fatal(err)
	}
	g.index()
	if err := g.writeGuides(); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"Rules/Teleportation", "Rules/Scribing", "Rules/Learning-Spells",
		"Rules/Survival", "Rules/Combat", "Rules/Day-and-Night"} {
		raw, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)+".md"))
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		page := string(raw)
		if !strings.HasPrefix(page, Marker) {
			t.Errorf("%s: missing the generated marker", rel)
		}
		for _, bad := range []string{"{", "<nil>", "%!"} {
			if strings.Contains(page, bad) {
				t.Errorf("%s: contains %q", rel, bad)
			}
		}
		if os.Getenv("WIKI_GUIDES_DUMP") != "" {
			t.Logf("──── %s ────\n%s", rel, page)
		}
	}
}
