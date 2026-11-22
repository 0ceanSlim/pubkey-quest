package imagegen

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// An approved candidate becomes the full badge and its thumbs.
func TestPublishBadge(t *testing.T) {
	dir := t.TempDir()
	cand := filepath.Join(dir, "_candidates", "every-fish")
	if err := os.MkdirAll(cand, 0o755); err != nil {
		t.Fatal(err)
	}
	src := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			src.Set(x, y, color.RGBA{uint8(x / 4), uint8(y / 4), 90, 255})
		}
	}
	f, _ := os.Create(filepath.Join(cand, "gpt-image-2-v2.png"))
	png.Encode(f, src)
	f.Close()

	files, err := PublishBadge(dir, "every-fish", "v2")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"every-fish.jpg": 1024, "every-fish-256.jpg": 256, "every-fish-64.jpg": 64}
	if len(files) != len(want) {
		t.Fatalf("wrote %v", files)
	}
	for _, p := range files {
		r, _ := os.Open(p)
		cfg, err := jpeg.DecodeConfig(r)
		r.Close()
		if err != nil || cfg.Width != want[filepath.Base(p)] || cfg.Height != cfg.Width {
			t.Errorf("%s: %dx%d %v", p, cfg.Width, cfg.Height, err)
		}
	}
	if _, err := PublishBadge(dir, "../escape", "v1"); err == nil {
		t.Error("a badge id with a path in it was accepted")
	}
}
