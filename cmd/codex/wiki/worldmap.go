package wiki

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ── The world map ────────────────────────────────────────────────────────────
//
// Derived from the road graph (roadmap C3): every road (environment) joins two
// city districts, and a district's compass key says which way the road leaves
// town. Laying cities out by those directions from the Kingdom gives a grid
// that matches how travel feels in game. It's drawn as an SVG (media/), and the
// Map page lists the roads and towns.

var biomeColor = map[string]string{
	"arctic": "#a5d8ff", "coast": "#4dabf7", "desert": "#e8b84a", "forest": "#2f9e44",
	"grassland": "#94d82d", "hill": "#c49a5a", "mountain": "#9aa1a9", "swamp": "#6b8f4e", "urban": "#d9a3f0",
}

var compass = map[string][2]int{"north": {0, -1}, "south": {0, 1}, "east": {1, 0}, "west": {-1, 0}}

type road struct {
	env        string
	a, b       string // city ids
	aDir, bDir string // the district each end leaves from
}

// roads splits each environment's two connect ids into city + district.
func (g *gen) roads() []road {
	var out []road
	for _, id := range sortedKeys(g.envs) {
		e := g.envs[id]
		if len(e.Connects) != 2 {
			continue
		}
		r := road{env: id}
		for i, c := range e.Connects {
			city := g.district[c]
			if city == "" {
				continue
			}
			dir := strings.TrimPrefix(c, city+"-")
			if i == 0 {
				r.a, r.aDir = city, dir
			} else {
				r.b, r.bDir = city, dir
			}
		}
		if r.a != "" && r.b != "" {
			out = append(out, r)
		}
	}
	return out
}

// layout places every city reachable from the start on a compass grid.
func (g *gen) layout(roads []road, start string) map[string][2]int {
	pos := map[string][2]int{start: {0, 0}}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, r := range roads {
			var from, to, dir string
			switch cur {
			case r.a:
				from, to, dir = r.a, r.b, r.aDir
			case r.b:
				from, to, dir = r.b, r.a, r.bDir
			default:
				continue
			}
			if _, done := pos[to]; done {
				continue
			}
			d, ok := compass[dir]
			if !ok {
				d = [2]int{1, 1}
			}
			p := pos[from]
			next := [2]int{p[0] + d[0], p[1] + d[1]}
			for taken(pos, next) { // never stack two towns on one spot
				next[0] += d[0] + boolInt(d[0] == 0)
				next[1] += d[1]
			}
			pos[to] = next
			queue = append(queue, to)
		}
	}
	return pos
}

func taken(pos map[string][2]int, p [2]int) bool {
	for _, q := range pos {
		if q == p {
			return true
		}
	}
	return false
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// writeMap writes media/world-map.svg and the Map page.
func (g *gen) writeMap() error {
	roads := g.roads()
	pos := g.layout(roads, "kingdom")
	if len(pos) == 0 {
		return nil
	}
	minX, minY, maxX, maxY := 0, 0, 0, 0
	for _, p := range pos {
		minX, minY = min(minX, p[0]), min(minY, p[1])
		maxX, maxY = max(maxX, p[0]), max(maxY, p[1])
	}
	const cellW, cellH, pad = 340, 200, 110
	px := func(id string) (int, int) {
		p := pos[id]
		return pad + (p[0]-minX)*cellW, pad + (p[1]-minY)*cellH
	}
	w, h := pad*2+(maxX-minX)*cellW, pad*2+(maxY-minY)*cellH
	base := strings.TrimRight(g.opt.BasePath, "/")

	waystones := map[string]bool{}
	for _, cid := range g.x.teleport.Chain {
		waystones[cid] = true
	}

	var s strings.Builder
	fmt.Fprintf(&s, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" font-family="monospace">`, w, h, w, h)
	fmt.Fprintf(&s, `<rect width="%d" height="%d" rx="12" fill="#1c1c22"/>`, w, h)
	// roads first, under the towns
	for _, r := range roads {
		if _, ok := pos[r.a]; !ok {
			continue
		}
		e := g.envs[r.env]
		x1, y1 := px(r.a)
		x2, y2 := px(r.b)
		col := biomeColor[e.EnvType]
		if col == "" {
			col = "#cccccc"
		}
		fmt.Fprintf(&s, `<a href="%s/Locations/%s"><line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="9" stroke-linecap="round" opacity="0.85"><title>%s</title></line>`,
			base, r.env, x1, y1, x2, y2, col, html.EscapeString(e.Name))
		// Two-line label: the road's name, then its crossing time and places.
		// Beside a north–south road, above an east–west one.
		mx, my := (x1+x2)/2, (y1+y2)/2
		detail := duration(e.TravelTime)
		if n := len(g.poisIn[r.env]); n > 0 {
			detail += fmt.Sprintf(" · %d %s", n, pluralWord(n, "place", "places"))
		}
		anchor, lx, ly := "middle", mx, my-26
		if x1 == x2 {
			anchor, lx, ly = "start", mx+16, my-6
		}
		fmt.Fprintf(&s, `<text x="%d" y="%d" fill="#e5e7eb" font-size="13" font-weight="bold" text-anchor="%s" paint-order="stroke" stroke="#1c1c22" stroke-width="4">%s</text>`,
			lx, ly, anchor, html.EscapeString(e.Name))
		fmt.Fprintf(&s, `<text x="%d" y="%d" fill="#9ca3af" font-size="11" text-anchor="%s" paint-order="stroke" stroke="#1c1c22" stroke-width="4">%s</text></a>`,
			lx, ly+15, anchor, html.EscapeString(detail))
	}
	// towns
	for _, cid := range sortedKeys(pos) {
		c := g.cities[cid]
		if c == nil {
			continue
		}
		x, y := px(cid)
		fill := "#2b2b36"
		if len(c.Races) > 0 {
			fill = "#3b3424"
		}
		fmt.Fprintf(&s, `<a href="%s/Locations/%s"><circle cx="%d" cy="%d" r="22" fill="%s" stroke="#fcd34d" stroke-width="3"><title>%s</title></circle>`,
			base, cid, x, y, fill, html.EscapeString(c.Name))
		if waystones[cid] {
			fmt.Fprintf(&s, `<text x="%d" y="%d" font-size="20" text-anchor="middle" dominant-baseline="central">🌀</text>`, x, y)
		}
		fmt.Fprintf(&s, `<text x="%d" y="%d" fill="#fcd34d" font-size="15" font-weight="bold" text-anchor="middle" paint-order="stroke" stroke="#1c1c22" stroke-width="4">%s</text></a>`,
			x, y+42, html.EscapeString(c.Name))
	}
	s.WriteString(`</svg>`)

	dst := filepath.Join(g.opt.OutDir, "media", "world-map.svg")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, []byte(s.String()), 0o644); err != nil {
		return err
	}
	g.res.Images++

	// The page: the map, then the roads and towns as tables.
	var b strings.Builder
	b.WriteString("The world as the roads join it: towns where travel begins and ends, roads coloured by terrain " +
		"with their crossing times. 🌀 marks a town square with a waystone (see " + link("Homeward Stones & Waystones", "Rules/Teleportation") + "). " +
		"Gold-ringed towns are starting cities.\n\n")
	painted, err := g.writePaintedMap()
	if err != nil {
		return err
	}
	if painted != "" {
		b.WriteString(painted + "\n\n## Route diagram\n\nThe same roads as a plain diagram, laid out by the compass directions you travel them.\n\n")
	}
	fmt.Fprintf(&b, `<img src="%s/media/world-map.svg?v=%s" alt="Route diagram" width="%d" class="world-map">`+"\n\n", base, fileFingerprint(dst), min(w, 900))

	var rows [][]string
	for _, r := range roads {
		e := g.envs[r.env]
		var places []string
		for _, p := range g.poisIn[r.env] {
			places = append(places, g.poiLink(p))
		}
		rows = append(rows, []string{link(e.Name, "Locations/"+r.env),
			link(g.cities[r.a].Name, "Locations/"+r.a) + " (" + r.aDir + ") ↔ " + link(g.cities[r.b].Name, "Locations/"+r.b) + " (" + r.bDir + ")",
			titleCase(e.EnvType), duration(e.TravelTime), strings.Join(places, ", ")})
	}
	b.WriteString("## Roads\n\n")
	table(&b, []string{"Road", "Between", "Terrain", "Crossing", "Places of interest"}, rows)

	rows = nil
	ids := sortedKeys(g.cities)
	sort.SliceStable(ids, func(i, j int) bool { return g.cities[ids[i]].Name < g.cities[ids[j]].Name })
	for _, cid := range ids {
		c := g.cities[cid]
		ws := ""
		if waystones[cid] {
			ws = "🌀"
		}
		var st []string
		for _, dk := range sortedKeys(c.Districts) {
			for _, bl := range c.Districts[dk].Buildings {
				for _, id := range bl.Stations {
					st = append(st, g.stationName(id))
				}
			}
		}
		rows = append(rows, []string{link(c.Name, "Locations/"+cid), strings.Join(c.Races, ", "), ws, strings.Join(st, ", ")})
	}
	b.WriteString("## Towns\n\n")
	table(&b, []string{"Town", "Home of", "Waystone", "Scribing"}, rows)
	return g.page("Map", "World Map", b.String())
}

func pluralWord(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
