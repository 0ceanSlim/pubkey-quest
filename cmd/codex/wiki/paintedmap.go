package wiki

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

// ── The painted world map with the game's places pinned on it ───────────────
//
// www/res/img/map/world-map.png is the painted map. Towns and roads carry a
// `map_position` (pixels on that painting) in their location JSON; the
// overlay pins each town (🌀 where a waystone stands) and labels each road
// with its crossing time, every pin a link. The painting is embedded as a
// JPEG data URI so the SVG works both as a plain <img> and inlined (where the
// pins are clickable).

const paintedMapSrc = "map/world-map.png" // under ImageDir

// writePaintedMap writes media/world-map-painted.svg and returns its <img>
// tag for the Map page, or "" when the painting or the positions are missing.
func (g *gen) writePaintedMap() (string, error) {
	src := filepath.Join(g.opt.ImageDir, filepath.FromSlash(paintedMapSrc))
	f, err := os.Open(src)
	if err != nil {
		return "", nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return "", nil
	}
	placed := 0
	for _, c := range g.cities {
		if len(c.MapPos) == 2 {
			placed++
		}
	}
	if placed == 0 {
		return "", nil
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, &jpeg.Options{Quality: 80}); err != nil {
		return "", err
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	base := strings.TrimRight(g.opt.BasePath, "/")
	waystones := map[string]bool{}
	for _, cid := range g.x.teleport.Chain {
		waystones[cid] = true
	}

	var s strings.Builder
	fmt.Fprintf(&s, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 %d %d" width="%d" height="%d" font-family="monospace">`, w, h, w, h)
	fmt.Fprintf(&s, `<image width="%d" height="%d" href="data:image/jpeg;base64,%s"/>`, w, h, base64.StdEncoding.EncodeToString(jpg.Bytes()))

	// Road labels: a dark chip with the name and the crossing time.
	for _, id := range sortedKeys(g.envs) {
		e := g.envs[id]
		if len(e.MapPos) != 2 {
			continue
		}
		x, y := int(e.MapPos[0]), int(e.MapPos[1])
		label := e.Name
		detail := duration(e.TravelTime)
		if n := len(g.poisIn[id]); n > 0 {
			detail += fmt.Sprintf(" · %d %s", n, pluralWord(n, "place", "places"))
		}
		wpx := max(len(label), len(detail))*8 + 16
		fmt.Fprintf(&s, `<a href="%s/Locations/%s"><g><title>%s</title>`, base, id, html.EscapeString(e.Name))
		fmt.Fprintf(&s, `<rect x="%d" y="%d" width="%d" height="38" rx="6" fill="#111118" fill-opacity="0.78" stroke="%s" stroke-width="2"/>`,
			x-wpx/2, y-19, wpx, colorOr(biomeColor[e.EnvType], "#cccccc"))
		fmt.Fprintf(&s, `<text x="%d" y="%d" fill="#f3f4f6" font-size="13" font-weight="bold" text-anchor="middle">%s</text>`, x, y-3, html.EscapeString(label))
		fmt.Fprintf(&s, `<text x="%d" y="%d" fill="#cbd5e1" font-size="11" text-anchor="middle">%s</text></g></a>`, x, y+12, html.EscapeString(detail))
	}

	// Town pins.
	for _, cid := range sortedKeys(g.cities) {
		c := g.cities[cid]
		if len(c.MapPos) != 2 {
			continue
		}
		x, y := int(c.MapPos[0]), int(c.MapPos[1])
		fmt.Fprintf(&s, `<a href="%s/Locations/%s"><g><title>%s</title>`, base, cid, html.EscapeString(c.Name))
		fmt.Fprintf(&s, `<circle cx="%d" cy="%d" r="17" fill="#111118" fill-opacity="0.4" stroke="#fcd34d" stroke-width="3"/>`, x, y)
		if waystones[cid] {
			fmt.Fprintf(&s, `<text x="%d" y="%d" font-size="18" text-anchor="middle" dominant-baseline="central">🌀</text>`, x, y)
		} else {
			fmt.Fprintf(&s, `<circle cx="%d" cy="%d" r="5" fill="#fcd34d"/>`, x, y)
		}
		lw := len(c.Name)*9 + 14
		fmt.Fprintf(&s, `<rect x="%d" y="%d" width="%d" height="22" rx="5" fill="#111118" fill-opacity="0.8"/>`, x-lw/2, y+22, lw)
		fmt.Fprintf(&s, `<text x="%d" y="%d" fill="#fcd34d" font-size="15" font-weight="bold" text-anchor="middle">%s</text></g></a>`,
			x, y+38, html.EscapeString(c.Name))
	}
	s.WriteString(`</svg>`)

	dst := filepath.Join(g.opt.OutDir, "media", "world-map-painted.svg")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, []byte(s.String()), 0o644); err != nil {
		return "", err
	}
	g.res.Images++
	return fmt.Sprintf(`<img src="%s/media/world-map-painted.svg?v=%s" alt="World map" width="900" class="world-map">`,
		base, fileFingerprint(dst)), nil
}

func colorOr(c, def string) string {
	if c == "" {
		return def
	}
	return c
}
