package imagegen

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// BadgeDir is where badge art is served from (/res/img/badges/), on the
// official site, so a DM badge definition's image URLs last as long as the game.
const BadgeDir = "www/res/img/badges"

// BadgeSizes are the images a badge definition (NIP-58 kind 30009) links: the
// full image and its thumbs. <id>.png is the first; the rest are <id>-<n>.png.
var BadgeSizes = []int{1024, 256, 64}

// PublishBadge turns an approved candidate into the badge's web images. The
// candidate is a variant name ("v2" → _candidates/<id>/gpt-image-2-v2.png) or a
// path. PNG, as a badge's background is transparent (the candidate was keyed
// out, keyout.go). It returns the files written.
func PublishBadge(dir, id, candidate string) ([]string, error) {
	if id == "" || strings.ContainsAny(id, `/\.`) {
		return nil, fmt.Errorf("bad badge id %q", id)
	}
	src := candidate
	if !strings.ContainsAny(candidate, `/\.`) {
		src = filepath.Join(dir, "_candidates", id, "gpt-image-2-"+candidate+".png")
	}
	f, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", src, err)
	}
	var out []string
	for i, n := range BadgeSizes {
		dst := image.NewRGBA(image.Rect(0, 0, n, n))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
		name := id + ".png"
		if i > 0 {
			name = fmt.Sprintf("%s-%d.png", id, n)
		}
		path := filepath.Join(dir, name)
		w, err := os.Create(path)
		if err != nil {
			return out, err
		}
		err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(w, dst)
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return out, err
		}
		out = append(out, path)
	}
	return out, nil
}
