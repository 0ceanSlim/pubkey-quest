package game

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // decode GIF portraits
	_ "image/jpeg" // decode JPEG portraits
	"image/png"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "golang.org/x/image/webp" // decode WebP portraits

	gamedata "pubkey-quest/cmd/server/api/data"
	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/session"
)

// ─── Character portraits (party-combat-plan §2c) ─────────────────────────────
//
// A portrait is a picture the player chose for this character — separate from
// their Nostr picture. It's one optional URL on the save (the player's choice,
// so it passes the hydration rule) and it shows on their grid token, the turn
// strip, the People column and the player card.
//
// Clients never load a portrait URL directly. /api/portrait fetches it once
// (only if it's the portrait of a character currently in the game), checks it
// is a real image under a size cap, scales it to a 64×64 PNG and serves that
// from memory. Other players' image hosts never see your IP, huge or animated
// files can't hurt the scene, and everything is crisp at token size.

const (
	portraitMaxURL   = 512
	portraitMaxBytes = 8 << 20
	portraitSize     = 64
	portraitCacheCap = 512
)

// PortraitRequest sets (or clears, with an empty url) the character's portrait.
// swagger:model PortraitRequest
type PortraitRequest struct {
	Npub   string `json:"npub"    example:"npub1..."`
	SaveID string `json:"save_id" example:"save_1234567890"`
	URL    string `json:"url"     example:"https://blossom.example/abc.png"`
}

// PortraitResponse confirms the portrait and gives the proxied image path.
// swagger:model PortraitResponse
type PortraitResponse struct {
	Success bool   `json:"success" example:"true"`
	URL     string `json:"url"     example:"https://blossom.example/abc.png"`
	Src     string `json:"src"     example:"/api/portrait?u=https%3A%2F%2Fblossom.example%2Fabc.png"`
	Message string `json:"message,omitempty"`
}

// PortraitSrc is the proxied image path for a portrait URL ("" for none).
func PortraitSrc(u string) string {
	if u == "" {
		return ""
	}
	return "/api/portrait?u=" + url.QueryEscape(u)
}

// validPortraitURL accepts an https URL of reasonable length.
func validPortraitURL(raw string) error {
	if len(raw) > portraitMaxURL {
		return fmt.Errorf("that link is too long")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("use an https:// link to an image")
	}
	return nil
}

// SetPortraitHandler godoc
// @Summary      Set the character's portrait
// @Description  Stores an https image URL as this character's portrait (empty clears it). Like everything
// @Description  on the save, it persists when the player next saves.
// @Tags         Game
// @Accept       json
// @Produce      json
// @Param        request  body      PortraitRequest  true  "Portrait"
// @Success      200      {object}  PortraitResponse
// @Failure      400      {string}  string  "Bad URL"
// @Router       /api/game/portrait [post]
func SetPortraitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req PortraitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Npub == "" || req.SaveID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub or save_id")
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL != "" {
		if err := validPortraitURL(req.URL); err != nil {
			writeCombatError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	sess, err := session.GetSessionManager().GetSession(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, "Session not found")
		return
	}
	// Check it really is an image before keeping it.
	if req.URL != "" {
		if _, err := portraitPNG(req.URL); err != nil {
			writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Couldn't use that image: %v", err))
			return
		}
	}
	sess.SaveData.Portrait = req.URL
	writeCombatJSON(w, http.StatusOK, PortraitResponse{
		Success: true, URL: req.URL, Src: PortraitSrc(req.URL),
		Message: "Portrait set — save your game to keep it.",
	})
}

// PortraitPromptResponse is an AI image prompt built from the character.
// swagger:model PortraitPromptResponse
type PortraitPromptResponse struct {
	Success bool   `json:"success" example:"true"`
	Prompt  string `json:"prompt"`
}

// PortraitPromptHandler godoc
// @Summary      An image prompt for this character's portrait
// @Description  Writes a prompt for any AI image tool from the save: race, class, background, alignment, level,
// @Description  worn armour and weapon, plus the player's own "look" notes, with a fixed style line so every
// @Description  portrait shares a look.
// @Tags         Game
// @Produce      json
// @Param        npub     query     string  true   "Nostr public key"
// @Param        save_id  query     string  true   "Save ID"
// @Param        look     query     string  false  "Hair, scars, colours… (not stored)"
// @Success      200      {object}  PortraitPromptResponse
// @Router       /api/game/portrait-prompt [get]
func PortraitPromptHandler(w http.ResponseWriter, r *http.Request) {
	npub, saveID := r.URL.Query().Get("npub"), r.URL.Query().Get("save_id")
	sess, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, "Session not found")
		return
	}
	look := strings.TrimSpace(r.URL.Query().Get("look"))
	if len(look) > 300 {
		look = look[:300]
	}
	writeCombatJSON(w, http.StatusOK, PortraitPromptResponse{Success: true, Prompt: portraitPrompt(sess, look)})
}

// portraitPrompt assembles the prompt text.
func portraitPrompt(sess *session.GameSession, look string) string {
	save := &sess.SaveData
	adv, _ := loadAdvancement()
	level := character.GetLevelFromXP(save.Experience, adv)

	itemName := func(slots ...string) string {
		id := getEquippedIDMulti(save.Inventory, slots...)
		if id == "" {
			return ""
		}
		if item, err := gamedata.LoadItemByID(serverdb.GetDB(), id); err == nil {
			if n, ok := item["name"].(string); ok && n != "" {
				return strings.ToLower(n)
			}
		}
		return strings.ReplaceAll(id, "-", " ")
	}

	var b strings.Builder
	b.WriteString("Square pixel-art bust portrait for a 16-bit fantasy RPG. ")
	who := fmt.Sprintf("A level %d %s %s", level, lowerWords(save.Race), lowerWords(save.Class))
	if save.Background != "" {
		who += ", " + lowerWords(save.Background) + " background"
	}
	if save.Alignment != "" {
		who += ", " + lowerWords(save.Alignment)
	}
	b.WriteString(who + ". ")
	var gear []string
	if a := itemName("armor", "chest"); a != "" {
		gear = append(gear, "wearing "+a)
	}
	if wpn := itemName("mainhand", "right_arm"); wpn != "" {
		gear = append(gear, "carrying a "+wpn)
	}
	if off := itemName("offhand", "left_arm"); off != "" {
		gear = append(gear, "with a "+off)
	}
	if len(gear) > 0 {
		g := strings.Join(gear, ", ")
		b.WriteString(strings.ToUpper(g[:1]) + g[1:] + ". ")
	}
	if look != "" {
		b.WriteString(strings.TrimRight(look, ". ") + ". ")
	}
	b.WriteString("Facing slightly left, plain dark background, no text, no border, crisp pixels.")
	return b.String()
}

// lowerWords turns "half-orc" / "Lawful Neutral" into prompt-friendly text.
func lowerWords(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "_", " "))
}

// ─── The image proxy ─────────────────────────────────────────────────────────

var (
	portraitMu    sync.Mutex
	portraitCache = map[string][]byte{} // sha256(url) → 64×64 PNG
	portraitOrder []string              // insertion order, for eviction
)

// PortraitImageHandler godoc
// @Summary      A character portrait, scaled for the game
// @Description  Serves a 64×64 PNG of a portrait URL that belongs to a character currently in the game.
// @Tags         Game
// @Produce      png
// @Param        u  query  string  true  "The portrait URL"
// @Success      200  {file}  binary
// @Failure      404  {string}  string  "Not a current portrait"
// @Router       /api/portrait [get]
func PortraitImageHandler(w http.ResponseWriter, r *http.Request) {
	u := r.URL.Query().Get("u")
	if u == "" || validPortraitURL(u) != nil || !isLivePortrait(u) {
		http.NotFound(w, r)
		return
	}
	img, err := portraitPNG(u)
	if err != nil {
		log.Printf("⚠️ portrait %s: %v", u, err)
		http.Error(w, "portrait unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(img)
}

// isLivePortrait reports whether some loaded character uses this portrait, so
// the proxy can't be pointed at arbitrary URLs.
func isLivePortrait(u string) bool {
	for _, s := range session.GetSessionManager().GetAllSessions() {
		if s.SaveData.Portrait == u {
			return true
		}
	}
	return false
}

// portraitPNG returns the cached 64×64 PNG for a portrait URL, fetching and
// scaling it on a miss.
func portraitPNG(u string) ([]byte, error) {
	sum := sha256.Sum256([]byte(u))
	key := hex.EncodeToString(sum[:])
	portraitMu.Lock()
	if b, ok := portraitCache[key]; ok {
		portraitMu.Unlock()
		return b, nil
	}
	portraitMu.Unlock()

	raw, err := fetchPortrait(u)
	if err != nil {
		return nil, err
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("not a PNG, JPEG, GIF or WebP image")
	}
	var out bytes.Buffer
	if err := png.Encode(&out, scaleSquare(src, portraitSize)); err != nil {
		return nil, err
	}

	portraitMu.Lock()
	defer portraitMu.Unlock()
	portraitCache[key] = out.Bytes()
	portraitOrder = append(portraitOrder, key)
	for len(portraitOrder) > portraitCacheCap {
		delete(portraitCache, portraitOrder[0])
		portraitOrder = portraitOrder[1:]
	}
	return out.Bytes(), nil
}

// portraitClient fetches portraits from the public internet only: it refuses to
// connect to loopback, private, link-local or otherwise internal addresses
// (checked on the resolved IP, so DNS can't smuggle one in).
var portraitClient = &http.Client{
	Timeout: 8 * time.Second,
	Transport: &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return err
				}
				ip := net.ParseIP(host)
				if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
					ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
					return errors.New("refusing to fetch from an internal address")
				}
				return nil
			},
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" {
			return errors.New("redirected off https")
		}
		return nil
	},
}

func fetchPortrait(u string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "PubkeyQuest-portrait/1")
	resp, err := portraitClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch it: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the host answered %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, portraitMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > portraitMaxBytes {
		return nil, fmt.Errorf("it's over 8 MB")
	}
	return raw, nil
}

// scaleSquare centre-crops an image to a square and box-filters it down to
// size×size (every source pixel contributes, so detail averages rather than
// aliasing).
func scaleSquare(src image.Image, size int) image.Image {
	b := src.Bounds()
	side := b.Dx()
	if b.Dy() < side {
		side = b.Dy()
	}
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for dy := 0; dy < size; dy++ {
		sy0 := y0 + dy*side/size
		sy1 := y0 + (dy+1)*side/size
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for dx := 0; dx < size; dx++ {
			sx0 := x0 + dx*side/size
			sx1 := x0 + (dx+1)*side/size
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var r, g, bl, a, n uint64
			for y := sy0; y < sy1; y++ {
				for x := sx0; x < sx1; x++ {
					cr, cg, cb, ca := src.At(x, y).RGBA()
					r, g, bl, a = r+uint64(cr), g+uint64(cg), bl+uint64(cb), a+uint64(ca)
					n++
				}
			}
			if a == 0 {
				dst.Set(dx, dy, color.NRGBA{})
				continue
			}
			// Average premultiplied channels, then un-premultiply.
			dst.Set(dx, dy, color.NRGBA{
				R: uint8((r * 0xff) / a),
				G: uint8((g * 0xff) / a),
				B: uint8((bl * 0xff) / a),
				A: uint8(a / n >> 8),
			})
		}
	}
	return dst
}
