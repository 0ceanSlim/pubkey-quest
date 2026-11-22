package imagegen

import (
	"image"
	"image/color"
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
	want := map[string]int{"every-fish.png": 1024, "every-fish-256.png": 256, "every-fish-64.png": 64}
	if len(files) != len(want) {
		t.Fatalf("wrote %v", files)
	}
	for _, p := range files {
		r, _ := os.Open(p)
		cfg, err := png.DecodeConfig(r)
		r.Close()
		if err != nil || cfg.Width != want[filepath.Base(p)] || cfg.Height != cfg.Width {
			t.Errorf("%s: %dx%d %v", p, cfg.Width, cfg.Height, err)
		}
	}
	if _, err := PublishBadge(dir, "../escape", "v1"); err == nil {
		t.Error("a badge id with a path in it was accepted")
	}
}

// The key colour around a badge goes; the same colour inside its outline stays.
func TestKeyOut(t *testing.T) {
	key := color.RGBA{255, 0, 255, 255}
	img := image.NewRGBA(image.Rect(0, 0, 9, 9))
	for y := range 9 {
		for x := range 9 {
			switch {
			case x >= 2 && x <= 6 && y >= 2 && y <= 6 && (x == 2 || x == 6 || y == 2 || y == 6):
				img.Set(x, y, color.Black) // the outline
			case x == 4 && y == 4:
				img.Set(x, y, key) // key colour inside the badge
			default:
				img.Set(x, y, key)
			}
		}
	}
	out := KeyOut(img, key)
	if out.NRGBAAt(0, 0).A != 0 || out.NRGBAAt(8, 4).A != 0 {
		t.Error("the background wasn't keyed out")
	}
	if out.NRGBAAt(2, 2).A != 255 || out.NRGBAAt(4, 4).A != 255 {
		t.Error("the badge itself lost pixels")
	}
}
