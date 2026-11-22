package imagegen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
)

// ─── Keying out a background ───────────────────────────────────────────────
//
// gpt-image-2 can't draw on transparency, so a cut-out (a badge) is drawn on a
// flat key colour (spec "key": "#ff00ff") and the key is removed here: a flood
// fill from the image's edge through pixels near the key. The subject's outline
// stops the fill, so the key colour inside the subject is left alone.

// keyReach is how far (summed RGB difference) a pixel may be from the key and
// still be background; keyFringe is the looser test for the soft pixels along
// the subject's edge, which are part key and part outline.
const (
	keyReach  = 120
	keyFringe = 260
)

// parseKey reads a "#rrggbb" colour.
func parseKey(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil || len(s) != 6 {
		return color.RGBA{}, fmt.Errorf("key %q: want #rrggbb", s)
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
}

func keyDist(c color.NRGBA, k color.RGBA) int {
	d := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	return d(c.R, k.R) + d(c.G, k.G) + d(c.B, k.B)
}

// KeyOut returns img with the key-coloured background around the subject made
// transparent.
func KeyOut(img image.Image, key color.RGBA) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			out.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	w, h := b.Dx(), b.Dy()
	bg := make([]bool, w*h)
	var stack []int
	push := func(x, y int) {
		if x < 0 || y < 0 || x >= w || y >= h || bg[y*w+x] {
			return
		}
		if keyDist(out.NRGBAAt(x, y), key) > keyReach {
			return
		}
		bg[y*w+x] = true
		stack = append(stack, y*w+x)
	}
	for x := 0; x < w; x++ {
		push(x, 0)
		push(x, h-1)
	}
	for y := 0; y < h; y++ {
		push(0, y)
		push(w-1, y)
	}
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := i%w, i/w
		push(x+1, y)
		push(x-1, y)
		push(x, y+1)
		push(x, y-1)
	}
	// The fringe: a pixel touching the background that's still more key than
	// subject goes too.
	fringe := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if bg[y*w+x] || keyDist(out.NRGBAAt(x, y), key) > keyFringe {
				continue
			}
			for _, n := range [][2]int{{x + 1, y}, {x - 1, y}, {x, y + 1}, {x, y - 1}} {
				if n[0] >= 0 && n[1] >= 0 && n[0] < w && n[1] < h && bg[n[1]*w+n[0]] {
					fringe[y*w+x] = true
					break
				}
			}
		}
	}
	for i := range bg {
		if bg[i] || fringe[i] {
			out.Pix[i*4+3] = 0
		}
	}
	// Despill: an edge pixel tinted by the key loses the tint (for magenta,
	// red and blue come down to green, so a purple-black outline turns black).
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			if out.Pix[i*4+3] == 0 {
				continue
			}
			edge := false
			for _, n := range [][2]int{{x + 1, y}, {x - 1, y}, {x, y + 1}, {x, y - 1}} {
				if n[0] >= 0 && n[1] >= 0 && n[0] < w && n[1] < h && out.Pix[(n[1]*w+n[0])*4+3] == 0 {
					edge = true
					break
				}
			}
			if !edge {
				continue
			}
			p := out.Pix[i*4 : i*4+3]
			if key.R > 128 && key.B > 128 && key.G < 128 && p[0] > p[1] && p[2] > p[1] {
				p[0], p[2] = p[1], p[1]
			}
		}
	}
	return out
}

// keyOutPNG keys a PNG's background out and re-encodes it.
func keyOutPNG(data []byte, key color.RGBA) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, KeyOut(img, key)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
