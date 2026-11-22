package dm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"

	"pubkey-quest/cmd/server/game/boards"
	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/types"
)

// The events the Dungeon Master signs (docs/draft/server-stats.md §5–7,
// competition-plan.md slice 5). Built here unsigned; dm.go signs and sends.

const (
	KindBadgeDefinition = 30009 // NIP-58
	KindBadgeAward      = 8     // NIP-58
	KindScore           = 30762 // Gamestr
	KindAppData         = 30078 // a player's lifetime record, mirrored

	// GameTag is the game's id on Gamestr and in every d tag.
	GameTag = "pubkey-quest"
	GameURL = "https://pubkey.quest"
)

// recordD is the d tag of a player's mirrored record.
func recordD(playerHex string) string { return GameTag + ":stats:" + playerHex }

// Definition is a badge's definition (kind 30009): its name, description and
// art on Blossom.
func Definition(b types.BadgeDef, m types.BadgeMedia) *nostr.Event {
	tags := [][]string{{"d", b.ID}, {"name", b.Name}, {"description", b.Description}}
	if m.Image != "" {
		tags = append(tags, []string{"image", m.Image, "1024x1024"})
	}
	if m.Thumb256 != "" {
		tags = append(tags, []string{"thumb", m.Thumb256, "256x256"})
	}
	if m.Thumb64 != "" {
		tags = append(tags, []string{"thumb", m.Thumb64, "64x64"})
	}
	return &nostr.Event{Kind: KindBadgeDefinition, CreatedAt: time.Now().Unix(), Tags: tags}
}

// Award is a badge awarded to a player (kind 8).
func Award(badgeID, dmHex, playerHex string) *nostr.Event {
	return &nostr.Event{Kind: KindBadgeAward, CreatedAt: time.Now().Unix(), Tags: [][]string{
		{"a", fmt.Sprintf("%d:%s:%s", KindBadgeDefinition, dmHex, badgeID)},
		{"p", playerHex},
	}}
}

// Score is a player's one score on Gamestr (kind 30762): their XP, which
// Gamestr ranks, with every other board along as a tag of its own and a line
// of the words. Gamestr keeps the newest. The boards themselves are ours
// (game/boards, the in-game Highscores).
func Score(r stats.Record, playerHex string) *nostr.Event {
	xp := r.Scores[boards.XP]
	lines := []string{ScoreLine(boards.XP, xp)}
	if xp.Who == "" { // no XP yet: say who from another board
		for _, b := range scoreBoards(r) {
			if s := r.Scores[b]; s.Who != "" {
				xp.Who, xp.Detail = s.Who, s.Detail
				lines[0] = ScoreLine(boards.XP, xp)
				break
			}
		}
	}
	tags := [][]string{
		{"d", GameTag + ":" + playerHex},
		{"game", GameTag},
		{"i", GameURL},
		{"k", "web"},
		{"score", strconv.FormatInt(xp.Value, 10)},
		{"p", playerHex},
		{"state", "active"},
		{"t", "rpg"},
	}
	for _, b := range scoreBoards(r) {
		tags = append(tags, []string{b, strconv.FormatInt(r.Scores[b].Value, 10)})
		lines = append(lines, ScoreLine(b, r.Scores[b]))
	}
	if len(r.Badges) > 0 {
		ids := make([]string, 0, len(r.Badges))
		for id := range r.Badges {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		tags = append(tags, []string{"achievements", strings.Join(ids, ",")})
	}
	return &nostr.Event{Kind: KindScore, CreatedAt: time.Now().Unix(), Content: strings.Join(lines, "\n"), Tags: tags}
}

// scoreBoards are the player's boards besides XP, in the order the score
// lists them: the fixed boards, then the biggest fish by species.
func scoreBoards(r stats.Record) []string {
	var out, fish []string
	for _, b := range []string{boards.NetWorth, boards.QuestPoints, boards.Places, boards.Badges,
		boards.Slain, boards.Toughest, boards.FishLanded} {
		if _, ok := r.Scores[b]; ok {
			out = append(out, b)
		}
	}
	for b := range r.Scores {
		if strings.HasPrefix(b, boards.Biggest) {
			fish = append(fish, b)
		}
	}
	sort.Strings(fish)
	return append(out, fish...)
}

// ScoreKey is what a player's Gamestr score carries, so it's signed again
// only when that changes: each board's value.
func ScoreKey(r stats.Record) string {
	parts := make([]string, 0, len(r.Scores))
	for b, s := range r.Scores {
		parts = append(parts, b+"="+strconv.FormatInt(s.Value, 10))
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

// Retired takes down a board's own score from before the DM posted one per
// player: the same event, its state "retired", which Gamestr hides.
func Retired(board string, s stats.Score, playerHex string) *nostr.Event {
	return &nostr.Event{Kind: KindScore, CreatedAt: time.Now().Unix(), Content: ScoreLine(board, s), Tags: [][]string{
		{"d", GameTag + ":" + playerHex + ":" + board},
		{"game", GameTag},
		{"i", GameURL},
		{"k", "web"},
		{"score", strconv.FormatInt(s.Value, 10)},
		{"p", playerHex},
		{"state", "retired"},
		{"level", board},
	}}
}

// Mirror is a player's lifetime record, published as the store of record.
func Mirror(r stats.Record, playerHex string) (*nostr.Event, error) {
	content, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return &nostr.Event{Kind: KindAppData, CreatedAt: time.Now().Unix(), Content: string(content), Tags: [][]string{
		{"d", recordD(playerHex)},
		{"p", playerHex},
	}}, nil
}

// ScoreLine is a score's words: "Mara, level 7 Druid: 23,400 XP."
func ScoreLine(board string, s stats.Score) string {
	v := thousands(s.Value)
	switch {
	case board == boards.XP:
		return fmt.Sprintf("%s, %s: %s XP.", s.Who, s.Detail, v)
	case board == boards.NetWorth:
		return fmt.Sprintf("%s, %s: worth %s gold.", s.Who, s.Detail, v)
	case board == boards.QuestPoints:
		return fmt.Sprintf("%s, %s: %s quest points.", s.Who, s.Detail, v)
	case board == boards.Places:
		return fmt.Sprintf("%s has found %s places in the realm.", s.Who, v)
	case board == boards.Badges:
		return fmt.Sprintf("%s badges from the Dungeon Master.", v)
	case board == boards.Slain:
		return fmt.Sprintf("%s monsters slain.", v)
	case board == boards.FishLanded:
		return fmt.Sprintf("%s fish landed.", v)
	case board == boards.Toughest:
		return fmt.Sprintf("%s slew %s, worth %s XP.", s.Who, withArticle(words(s.Detail)), v)
	case strings.HasPrefix(board, boards.Biggest):
		return fmt.Sprintf("%s landed a %s %s.", s.Who, lbOz(s.Value), words(s.Detail))
	}
	return fmt.Sprintf("%s: %s.", s.Who, v)
}

// thousands writes 23400 as "23,400".
func thousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		return "-" + s
	}
	return s
}

// lbOz writes ounces as "19 lb 6 oz".
func lbOz(oz int64) string {
	switch lb, rest := oz/16, oz%16; {
	case lb == 0:
		return fmt.Sprintf("%d oz", rest)
	case rest == 0:
		return fmt.Sprintf("%d lb", lb)
	default:
		return fmt.Sprintf("%d lb %d oz", lb, rest)
	}
}

// words turns an id into words: "river-pike" → "river pike".
func words(id string) string { return strings.ReplaceAll(id, "-", " ") }

func withArticle(s string) string {
	if s == "" {
		return s
	}
	if strings.ContainsRune("aeiou", rune(s[0])) {
		return "an " + s
	}
	return "a " + s
}
