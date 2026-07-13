package game

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"pubkey-quest/cmd/server/api/data"
	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

// ============================================================================
// Request / Response types
// ============================================================================

// AbilityPointsResponse reports a character's spendable ability points and the
// six current ability scores. Returned by GET /api/progression/ability-points.
type AbilityPointsResponse struct {
	Success bool           `json:"success"`
	Unspent int            `json:"unspent"`
	Cap     int            `json:"cap"`
	Scores  map[string]int `json:"scores"`
}

// SpendAbilityPointRequest spends one banked point into an ability.
type SpendAbilityPointRequest struct {
	Npub    string `json:"npub"`
	SaveID  string `json:"save_id"`
	Ability string `json:"ability"` // full name or 3-letter abbrev, any case (e.g. "Strength" / "str")
}

// SpendAbilityPointResponse is returned after a successful allocation.
type SpendAbilityPointResponse struct {
	Success bool           `json:"success"`
	Ability string         `json:"ability"`
	Unspent int            `json:"unspent"`
	Cap     int            `json:"cap"`
	Scores  map[string]int `json:"scores"`
	MaxHP   int            `json:"max_hp"`
	MaxMana int            `json:"max_mana"`
}

// ============================================================================
// Handlers
// ============================================================================

// GetAbilityPointsHandler godoc
// @Summary      Get ability-point allocation state
// @Description  Returns the character's unspent ability points (derived from level,
//               feats taken, and points already allocated), the per-ability cap, and
//               the six current ability scores.
// @Tags         Progression
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save ID"
// @Success      200      {object}  AbilityPointsResponse
// @Failure      404      {string}  string  "Session not found"
// @Router       /api/progression/ability-points [get]
func GetAbilityPointsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		http.Error(w, "Missing query params: npub, save_id", http.StatusBadRequest)
		return
	}

	sess := getSessionAndValidate(w, npub, saveID)
	if sess == nil {
		return
	}

	adv, err := character.LoadAdvancement(serverdb.GetDB())
	if err != nil {
		http.Error(w, "Failed to load advancement data", http.StatusInternalServerError)
		return
	}

	save := sess.GetSaveData()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AbilityPointsResponse{
		Success: true,
		Unspent: character.UnspentAbilityPoints(save, adv),
		Cap:     character.AbilityScoreMax,
		Scores:  character.AbilityScores(save),
	})
}

// SpendAbilityPointHandler godoc
// @Summary      Spend one ability point
// @Description  Allocates one banked ability point into the named ability, raising its
//               score by 1 (capped at 20) and recording the choice. Bumping CON or a
//               casting stat re-derives Max HP / Mana. Blocked during active combat.
// @Tags         Progression
// @Accept       json
// @Produce      json
// @Param        request  body      SpendAbilityPointRequest  true  "Ability to raise"
// @Success      200      {object}  SpendAbilityPointResponse
// @Failure      400      {string}  string  "No points, at cap, or invalid ability"
// @Failure      404      {string}  string  "Session not found"
// @Failure      409      {string}  string  "In active combat"
// @Router       /api/progression/spend-point [post]
func SpendAbilityPointHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SpendAbilityPointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.Ability == "" {
		http.Error(w, "Missing required fields: npub, save_id, ability", http.StatusBadRequest)
		return
	}

	sess := getSessionAndValidate(w, req.Npub, req.SaveID)
	if sess == nil {
		return
	}
	if sess.ActiveCombat != nil {
		http.Error(w, "Cannot allocate ability points during combat", http.StatusConflict)
		return
	}

	adv, err := character.LoadAdvancement(serverdb.GetDB())
	if err != nil {
		http.Error(w, "Failed to load advancement data", http.StatusInternalServerError)
		return
	}

	save := sess.GetSaveData()
	if err := character.SpendAbilityPoint(save, req.Ability, adv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	canon := character.CanonicalAbility(req.Ability)
	log.Printf("✨ Ability point spent: %s → %d (unspent %d) for %s",
		canon, character.AbilityScores(save)[canon], character.UnspentAbilityPoints(save, adv), req.Npub)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SpendAbilityPointResponse{
		Success: true,
		Ability: canon,
		Unspent: character.UnspentAbilityPoints(save, adv),
		Cap:     character.AbilityScoreMax,
		Scores:  character.AbilityScores(save),
		MaxHP:   save.MaxHP,
		MaxMana: save.MaxMana,
	})
}

// LevelGuideResponse is the full 1→20 progression preview for a character.
type LevelGuideResponse struct {
	Success      bool                   `json:"success"`
	Class        string                 `json:"class"`
	CurrentLevel int                    `json:"current_level"`
	XPCurrent    int                    `json:"xp_current"`
	XPToNext     int                    `json:"xp_to_next"` // XP remaining to the next level (0 at 20)
	Unspent      int                    `json:"unspent"`    // ability points banked but not yet allocated
	Levels       []character.GuideLevel `json:"levels"`
}

// GetLevelGuideHandler godoc
// @Summary      Level-up progression guide
// @Description  Returns the character's full 1→20 path: XP thresholds, ability points,
//               feat-eligible levels, proficiency, HP/Mana, and class-specific martial
//               ability unlocks / caster spell-slot unlocks at each level. Read-only.
// @Tags         Progression
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save ID"
// @Success      200      {object}  LevelGuideResponse
// @Failure      404      {string}  string  "Session not found"
// @Router       /api/progression/guide [get]
func GetLevelGuideHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		http.Error(w, "Missing query params: npub, save_id", http.StatusBadRequest)
		return
	}

	sess := getSessionAndValidate(w, npub, saveID)
	if sess == nil {
		return
	}

	adv, err := character.LoadAdvancement(serverdb.GetDB())
	if err != nil {
		http.Error(w, "Failed to load advancement data", http.StatusInternalServerError)
		return
	}
	save := sess.GetSaveData()

	// Martial ability unlocks (empty for non-martial classes).
	guideAbilities := loadGuideAbilities(save.Class)

	// Caster spell-slot unlocks (nil for non-casters).
	var spellSlots map[int]map[string]int
	if character.IsCaster(save.Class) {
		if slots, err := loadClassSpellSlots(save.Class); err != nil {
			log.Printf("⚠️ guide: spell slots load failed for %s: %v", save.Class, err)
		} else {
			spellSlots = slots
		}
	}

	levels := character.BuildLevelGuide(save, adv, guideAbilities, spellSlots)
	currentLevel := character.GetLevelFromXP(save.Experience, adv)

	xpToNext := 0
	for _, e := range adv {
		if e.Level == currentLevel+1 {
			xpToNext = e.ExperiencePoints - save.Experience
			break
		}
	}
	if xpToNext < 0 {
		xpToNext = 0
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LevelGuideResponse{
		Success:      true,
		Class:        save.Class,
		CurrentLevel: currentLevel,
		XPCurrent:    save.Experience,
		XPToNext:     xpToNext,
		Unspent:      character.UnspentAbilityPoints(save, adv),
		Levels:       levels,
	})
}

// loadGuideAbilities loads a class's martial abilities and maps them to the slim
// shape BuildLevelGuide consumes. Returns empty for non-martial classes.
func loadGuideAbilities(class string) []character.GuideAbilityUnlock {
	abilities, err := data.LoadAbilitiesForClass(strings.ToLower(class))
	if err != nil {
		log.Printf("⚠️ guide: abilities load failed for %s: %v", class, err)
		return nil
	}
	out := make([]character.GuideAbilityUnlock, 0, len(abilities))
	for _, ab := range abilities {
		tiers := make([]character.GuideAbilityTier, 0, len(ab.ScalingTiers))
		for _, t := range ab.ScalingTiers {
			tiers = append(tiers, character.GuideAbilityTier{Level: t.MinLevel, Summary: t.Summary})
		}
		out = append(out, character.GuideAbilityUnlock{
			Name:        ab.Name,
			UnlockLevel: ab.UnlockLevel,
			Tiers:       tiers,
		})
	}
	return out
}

// loadClassSpellSlots reads the per-level spell-slot table for a caster class
// from the spell_slots_progression cache (built from spell-slots.json).
func loadClassSpellSlots(class string) (map[int]map[string]int, error) {
	database := serverdb.GetDB()
	if database == nil {
		return nil, fmt.Errorf("database not available")
	}
	var blob string
	if err := database.QueryRow("SELECT data FROM spell_slots_progression WHERE id = 'spell-slots'").Scan(&blob); err != nil {
		return nil, err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(blob), &top); err != nil {
		return nil, err
	}
	raw, ok := top[strings.ToLower(class)]
	if !ok {
		return nil, nil
	}
	var byLevel map[string]map[string]int
	if err := json.Unmarshal(raw, &byLevel); err != nil {
		return nil, err
	}
	out := make(map[int]map[string]int, len(byLevel))
	for lvlStr, slots := range byLevel {
		if lvl, err := strconv.Atoi(lvlStr); err == nil {
			out[lvl] = slots
		}
	}
	return out, nil
}

// ── Feats (M5.5) ─────────────────────────────────────────────────────────────

// FeatView is a feat definition plus whether this character has taken it.
type FeatView struct {
	types.Feat
	Taken  bool   `json:"taken"`
	Choice string `json:"choice,omitempty"` // the chosen stat, if already taken as a half-feat
}

// FeatsResponse lists the feats a character can see, plus their remaining feat slots.
type FeatsResponse struct {
	Success        bool       `json:"success"`
	SlotsAvailable int        `json:"slots_available"`
	EligibleLevels []int      `json:"eligible_levels"`
	Feats          []FeatView `json:"feats"`
}

// GetFeatsHandler godoc
// @Summary      List feats + this character's feat slots
// @Tags         Progression
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save ID"
// @Success      200      {object}  FeatsResponse
// @Router       /api/progression/feats [get]
func GetFeatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		http.Error(w, "Missing query params: npub, save_id", http.StatusBadRequest)
		return
	}
	sess := getSessionAndValidate(w, npub, saveID)
	if sess == nil {
		return
	}
	adv, err := character.LoadAdvancement(serverdb.GetDB())
	if err != nil {
		http.Error(w, "Failed to load advancement data", http.StatusInternalServerError)
		return
	}
	feats, err := data.LoadFeats()
	if err != nil {
		http.Error(w, "Failed to load feats", http.StatusInternalServerError)
		return
	}
	save := sess.GetSaveData()

	taken := make(map[string]string, len(save.FeatsChosen))
	for _, f := range save.FeatsChosen {
		id, choice := character.FeatBaseID(f)
		taken[id] = choice
	}
	views := make([]FeatView, 0, len(feats))
	for _, ft := range feats {
		choice, isTaken := taken[ft.ID]
		views = append(views, FeatView{Feat: ft, Taken: isTaken, Choice: choice})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(FeatsResponse{
		Success:        true,
		SlotsAvailable: character.FeatSlotsAvailable(save, adv),
		EligibleLevels: character.FeatLevelsSorted(save.Class),
		Feats:          views,
	})
}

// ChooseFeatRequest picks a feat, with a stat choice for half-feats.
type ChooseFeatRequest struct {
	Npub   string `json:"npub"`
	SaveID string `json:"save_id"`
	FeatID string `json:"feat_id"`
	Choice string `json:"choice,omitempty"` // stat for half-feats (e.g. "constitution")
}

// ChooseFeatHandler godoc
// @Summary      Take a feat (spends a feat slot instead of an ability point)
// @Tags         Progression
// @Accept       json
// @Produce      json
// @Param        request  body      ChooseFeatRequest  true  "Feat to take"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {string}  string  "No slot, already taken, prereq, or bad choice"
// @Failure      409      {string}  string  "In active combat"
// @Router       /api/progression/choose-feat [post]
func ChooseFeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ChooseFeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.FeatID == "" {
		http.Error(w, "Missing required fields: npub, save_id, feat_id", http.StatusBadRequest)
		return
	}
	sess := getSessionAndValidate(w, req.Npub, req.SaveID)
	if sess == nil {
		return
	}
	if sess.ActiveCombat != nil {
		http.Error(w, "Cannot choose a feat during combat", http.StatusConflict)
		return
	}
	adv, err := character.LoadAdvancement(serverdb.GetDB())
	if err != nil {
		http.Error(w, "Failed to load advancement data", http.StatusInternalServerError)
		return
	}
	feat, err := data.LoadFeatByID(req.FeatID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	save := sess.GetSaveData()
	if err := character.ChooseFeat(save, feat, req.Choice, adv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("✨ Feat taken: %s (choice=%q) for %s — %d slots left, %d unspent points",
		feat.Name, req.Choice, req.Npub, character.FeatSlotsAvailable(save, adv), character.UnspentAbilityPoints(save, adv))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"feat":            feat.Name,
		"chosen":          save.FeatsChosen,
		"slots_available": character.FeatSlotsAvailable(save, adv),
		"unspent":         character.UnspentAbilityPoints(save, adv),
		"scores":          character.AbilityScores(save),
		"max_hp":          save.MaxHP,
	})
}
