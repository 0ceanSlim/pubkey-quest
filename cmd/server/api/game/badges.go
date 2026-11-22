package game

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"pubkey-quest/cmd/server/dm"
	"pubkey-quest/cmd/server/game/badges"
	"pubkey-quest/cmd/server/game/boards"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/types"
)

// BadgeView is one badge as the journal's Badges tab shows it.
type BadgeView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Rim         string `json:"rim"`
	Rank        string `json:"rank,omitempty"` // a series shown by its highest earned
	Image       string `json:"image"`          // the 256 thumb, served by the game
	EarnedAt    int64  `json:"earned_at"`      // 0 = not yet
	Awarded     bool   `json:"awarded"`        // the DM's signed award is out (official)
	Have        int64  `json:"have,omitempty"` // progress toward it, where it has some
	Of          int64  `json:"of,omitempty"`
}

// BadgesHandler lists every badge, which the player has earned, and how far
// they are toward the rest.
//
// @Router /api/badges [get]
func BadgesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	npub, saveID := r.URL.Query().Get("npub"), r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		http.Error(w, "Missing query params: npub, save_id", http.StatusBadRequest)
		return
	}
	sess := getSessionAndValidate(w, npub, saveID)
	if sess == nil {
		return
	}
	rec, _ := stats.Load(npub)
	save := sess.GetSaveData()
	level := 0
	if adv, err := loadAdvancement(); err == nil {
		level = character.GetLevelFromXP(save.Experience, adv)
	}
	catalog := badges.Catalog()
	var views []BadgeView
	for _, b := range badges.Config().Badges {
		v := BadgeView{ID: b.ID, Name: b.Name, Description: b.Description, Rim: b.Rim, Rank: b.Rank,
			Image: "/res/img/badges/" + b.ID + "-256.png", EarnedAt: rec.Badges[b.ID], Awarded: rec.Awarded[b.ID] != ""}
		v.Have, v.Of = progress(b.Rule, catalog, &rec, save, level)
		views = append(views, v)
	}
	writeData(w, map[string]any{
		"enabled": stats.Enabled,
		"signed":  dm.Signs(), // official: earned badges become the DM's Nostr awards
		"badges":  views,
	})
}

// progress is how far a player is toward a badge: have of what it takes
// (0, 0 for a quest, which is done or not).
func progress(rule types.BadgeRule, catalog map[string][]string, r *stats.Record, save *types.SaveFile, level int) (int64, int64) {
	switch {
	case rule.Collection != "":
		all := catalog[rule.Collection]
		var have int64
		for _, m := range all {
			if badges.Earned(types.BadgeRule{Collection: rule.Collection}, map[string][]string{rule.Collection: {m}}, r, save, level) {
				have++
			}
		}
		return have, int64(len(all))
	case rule.Counter != "":
		return min(r.Counters[rule.Counter], rule.At), rule.At
	case rule.Level > 0:
		return int64(min(level, rule.Level)), int64(rule.Level)
	}
	return 0, 0
}

// BoardView names a board for the highscores page.
type BoardView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ScoreView is one place on a board.
type ScoreView struct {
	Rank  int    `json:"rank"`
	Npub  string `json:"npub"`
	Name  string `json:"name"`
	Value int64  `json:"value"`
	Line  string `json:"line"`
	You   bool   `json:"you,omitempty"`
}

// HighscoresHandler is a board's top scores (default XP), and every board
// there is. Anyone can read it.
//
// @Router /api/highscores [get]
func HighscoresHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	board := q.Get("board")
	if board == "" {
		board = boards.XP
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var entries []ScoreView
	for i, s := range stats.Top(board, limit) {
		name := s.Who
		if name == "" {
			name = s.Name
		}
		entries = append(entries, ScoreView{Rank: i + 1, Npub: s.Npub, Name: name, Value: s.Value,
			Line: dm.ScoreLine(board, s.Score), You: s.Npub == q.Get("npub")})
	}
	writeData(w, map[string]any{"board": board, "boards": boardList(), "entries": entries})
}

// boardList is every board, the headline first.
func boardList() []BoardView {
	out := []BoardView{
		{boards.XP, "Experience"}, {boards.NetWorth, "Net worth"}, {boards.QuestPoints, "Quest points"},
		{boards.Places, "Places found"}, {boards.Badges, "Badges"}, {boards.Slain, "Monsters slain"},
		{boards.Toughest, "Toughest kill"}, {boards.FishLanded, "Fish landed"},
	}
	for _, species := range badges.Catalog()[types.CollectionFish] {
		out = append(out, BoardView{boards.Biggest + species, "Biggest " + strings.ReplaceAll(species, "-", " ")})
	}
	return out
}

func writeData(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}
