package imagegen

import (
	"image"
	"image/color"
	"sort"
)

// ─── Palette quantizing ─────────────────────────────────────────────────────
//
// Badge PNGs are cut to a 256-colour palette (median cut, alpha included), which
// takes a 1024 badge from about 1 MB to under 200 KB without a visible change:
// the art is pixel art, so it has few real colours.

type qColor struct {
	c [4]uint8 // R, G, B, A (non-premultiplied)
	n int      // how many pixels have it
}

// Quantize returns img reduced to at most n colours.
func Quantize(img image.Image, n int) *image.Paletted {
	b := img.Bounds()
	counts := map[[4]uint8]int{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			counts[key4(img.At(x, y))]++
		}
	}
	all := make([]qColor, 0, len(counts))
	for c, k := range counts {
		all = append(all, qColor{c, k})
	}

	boxes := [][]qColor{all}
	for len(boxes) < n {
		// Split the box with the most error (pixels × squared distance from
		// its mean), along its most varied channel: a small patch of grey
		// gets its own colour instead of sharing one with a big blue.
		best, bestScore, bestCh := -1, 0.0, 0
		for i, box := range boxes {
			if len(box) < 2 {
				continue
			}
			ch, err := boxError(box)
			if err > bestScore {
				best, bestScore, bestCh = i, err, ch
			}
		}
		if best < 0 {
			break
		}
		box := boxes[best]
		sort.Slice(box, func(i, j int) bool { return box[i].c[bestCh] < box[j].c[bestCh] })
		total := 0
		for _, q := range box {
			total += q.n
		}
		cut, acc := 1, 0
		for i, q := range box {
			if acc += q.n; acc*2 >= total {
				cut = i + 1
				break
			}
		}
		if cut >= len(box) {
			cut = len(box) - 1
		}
		boxes[best] = box[:cut]
		boxes = append(boxes, box[cut:])
	}

	pal := make(color.Palette, len(boxes))
	for i, box := range boxes {
		var s [4]int
		w := 0
		for _, q := range box {
			for ch := range 4 {
				s[ch] += int(q.c[ch]) * q.n
			}
			w += q.n
		}
		pal[i] = color.NRGBA{uint8(s[0] / w), uint8(s[1] / w), uint8(s[2] / w), uint8(s[3] / w)}
	}

	// Each colour maps to its nearest palette entry.
	idx := make(map[[4]uint8]uint8, len(counts))
	for c := range counts {
		bi, bd := 0, -1
		for i, p := range pal {
			pc := p.(color.NRGBA)
			d := sq(int(c[0])-int(pc.R)) + sq(int(c[1])-int(pc.G)) + sq(int(c[2])-int(pc.B)) + sq(int(c[3])-int(pc.A))
			if bd < 0 || d < bd {
				bi, bd = i, d
			}
		}
		idx[c] = uint8(bi)
	}
	out := image.NewPaletted(image.Rect(0, 0, b.Dx(), b.Dy()), pal)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.SetColorIndex(x-b.Min.X, y-b.Min.Y, idx[key4(img.At(x, y))])
		}
	}
	return out
}

func key4(c color.Color) [4]uint8 {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	if n.A == 0 {
		return [4]uint8{} // every fully transparent pixel is one colour
	}
	return [4]uint8{n.R, n.G, n.B, n.A}
}

// boxError is a box's total squared error from its mean, and the channel
// that holds most of it.
func boxError(box []qColor) (int, float64) {
	var sum [4]float64
	w := 0.0
	for _, q := range box {
		for ch := range 4 {
			sum[ch] += float64(q.c[ch]) * float64(q.n)
		}
		w += float64(q.n)
	}
	var e [4]float64
	for _, q := range box {
		for ch := range 4 {
			d := float64(q.c[ch]) - sum[ch]/w
			e[ch] += d * d * float64(q.n)
		}
	}
	ch, total := 0, 0.0
	for i := range 4 {
		total += e[i]
		if e[i] > e[ch] {
			ch = i
		}
	}
	return ch, total
}

func sq(v int) int { return v * v }
