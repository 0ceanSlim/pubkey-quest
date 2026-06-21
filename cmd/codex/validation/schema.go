// Schema validation for POI/Encounter/Quest draft JSON files.
//
// Two passes:
//  1. Strict shape check — files parse cleanly into the canonical types
//     in package types with DisallowUnknownFields.
//  2. Reference + skill check — every monster_id, item id, effect id,
//     npc id, location id, quest id, and POI id resolves to a canonical
//     entry in game-data, and every skill name is one of the eight
//     derived skills.
//
// Lifted from cmd/schemacheck (now removed) so Codex is the single
// validation surface for game data.
package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pubkey-quest/types"
)

// SchemaDirs holds the source directories the schema validator scans.
// Defaults match the current draft layout; tests / future callers can override.
type SchemaDirs struct {
	POIs       string
	Encounters string
	Quests     string
	NPCs       string // external NPC tree (e.g. game-data/npcs)
	Items      string
	Monsters   string
	Effects    string
	Cities     string
	Environs   string
}

// DefaultSchemaDirs returns the canonical layout used by the game.
func DefaultSchemaDirs() SchemaDirs {
	return SchemaDirs{
		POIs:       "game-data/locations/poi-draft",
		Encounters: "game-data/systems/encounters-draft",
		Quests:     "game-data/quests-drafts",
		NPCs:       "game-data/npcs",
		Items:      "game-data/items",
		Monsters:   "game-data/monsters",
		Effects:    "game-data/effects",
		Cities:     "game-data/locations/cities",
		Environs:   "game-data/locations/environments",
	}
}

// Eight derived skills (see docs/poi-quest-design.md §7).
var schemaValidSkills = map[string]bool{
	"athletics": true, "crafting": true, "influence": true, "medicine": true,
	"perception": true, "resolve": true, "survival": true, "thieving": true,
}

var schemaValidStats = map[string]bool{
	"strength": true, "dexterity": true, "constitution": true,
	"intelligence": true, "wisdom": true, "charisma": true,
}

func schemaDecodeStrict[T any](path string, v *T) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func schemaWalkJSON(dir string) []string {
	var files []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".json") {
			files = append(files, p)
		}
		return nil
	})
	return files
}

// schemaLoadIDs reads every JSON file under dir and returns the set of
// top-level "id" values.
func schemaLoadIDs(dir string) map[string]bool {
	ids := map[string]bool{}
	for _, p := range schemaWalkJSON(dir) {
		var blob struct {
			ID string `json:"id"`
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if json.Unmarshal(b, &blob) == nil && blob.ID != "" {
			ids[blob.ID] = true
		}
	}
	return ids
}

// schemaRefIndex holds the resolved ID sets used during ref-checking.
type schemaRefIndex struct {
	items     map[string]bool
	monsters  map[string]bool
	effects   map[string]bool
	npcs      map[string]bool // external NPCs + embedded encounter NPCs
	npcHomes  map[string]string
	locations map[string]bool // cities + environments + POIs (homes for NPCs)
	quests    map[string]bool
	pois      map[string]bool
}

func buildSchemaRefIndex(dirs SchemaDirs) *schemaRefIndex {
	idx := &schemaRefIndex{
		items:     schemaLoadIDs(dirs.Items),
		monsters:  schemaLoadIDs(dirs.Monsters),
		effects:   schemaLoadIDs(dirs.Effects),
		npcs:      map[string]bool{},
		npcHomes:  map[string]string{},
		locations: map[string]bool{},
		quests:    schemaLoadIDs(dirs.Quests),
		pois:      schemaLoadIDs(dirs.POIs),
	}
	for k := range schemaLoadIDs(dirs.Cities) {
		idx.locations[k] = true
	}
	for k := range schemaLoadIDs(dirs.Environs) {
		idx.locations[k] = true
	}
	// External NPCs in game-data/npcs/<home_id>/<npc>.json.
	// Use lenient decode here so pre-existing unknown fields in city NPC
	// files (location_type/show_config) don't block the index build; the
	// strict shape check for external NPCs runs separately in
	// ValidateSchema and only enforces the new (id, primary_home) invariants.
	for _, p := range schemaWalkJSON(dirs.NPCs) {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var n types.NPCData
		if err := json.Unmarshal(b, &n); err != nil {
			continue
		}
		if n.ID != "" {
			idx.npcs[n.ID] = true
			idx.npcHomes[n.ID] = n.PrimaryHome
		}
	}
	// Embedded encounter NPCs (encounters intentionally keep inline NPCs).
	for _, p := range schemaWalkJSON(dirs.Encounters) {
		var v types.EncounterData
		if err := schemaDecodeStrict(p, &v); err != nil {
			continue
		}
		for _, n := range v.NPCs {
			if n.ID != "" {
				idx.npcs[n.ID] = true
			}
		}
	}
	return idx
}

type schemaChecker struct {
	idx     *schemaRefIndex
	path    string
	context string
	errs    []string
	warns   []string
}

func (c *schemaChecker) errf(format string, a ...any) {
	c.errs = append(c.errs, fmt.Sprintf("[%s] "+format, append([]any{c.context}, a...)...))
}

func (c *schemaChecker) warnf(format string, a ...any) {
	c.warns = append(c.warns, fmt.Sprintf("[%s] "+format, append([]any{c.context}, a...)...))
}

func (c *schemaChecker) checkRef(kind, id string, set map[string]bool) {
	if id == "" {
		return
	}
	if !set[id] {
		c.errf("unknown %s: %q", kind, id)
	}
}

// checkNPCRef warns instead of erroring when the NPC is missing — missing
// NPCs are an authoring TODO (the NPC file needs to be created), not a
// schema bug. Item/monster/location/effect/quest/poi refs stay hard errors.
func (c *schemaChecker) checkNPCRef(id string) {
	if id == "" {
		return
	}
	if !c.idx.npcs[id] {
		c.warnf("npc %q does not exist — needs to be created", id)
	}
}

func (c *schemaChecker) checkSkill(skill string) {
	if skill == "" {
		return
	}
	if !schemaValidSkills[skill] {
		c.errf("invalid skill %q (must be one of athletics/crafting/influence/medicine/perception/resolve/survival/thieving)", skill)
	}
}

func (c *schemaChecker) checkRequirements(reqs []types.POIRequirement) {
	for _, r := range reqs {
		switch r.Type {
		case "skill":
			c.checkSkill(r.ID)
		case "stat":
			if !schemaValidStats[r.ID] {
				c.errf("invalid stat %q in stat requirement", r.ID)
			}
		case "item":
			c.checkRef("item", r.ID, c.idx.items)
		case "quest_completed":
			c.checkRef("quest", r.ID, c.idx.quests)
		case "level", "quest_points":
			// numeric only — Min carries the value
		case "class", "race", "alignment":
			if len(r.Values) == 0 {
				c.errf("%s requirement missing values[]", r.Type)
			}
		default:
			c.errf("unknown requirement type %q", r.Type)
		}
	}
}

func (c *schemaChecker) checkLootTable(lt *types.POILootTable) {
	if lt == nil {
		return
	}
	for _, e := range lt.Guaranteed {
		c.checkRef("item", e.Item, c.idx.items)
	}
	for _, t := range lt.Tiers {
		for _, e := range t.Entries {
			c.checkRef("item", e.Item, c.idx.items)
		}
	}
}

func (c *schemaChecker) checkReward(r *types.POIReward) {
	if r == nil {
		return
	}
	for _, it := range r.Items {
		c.checkRef("item", it.ID, c.idx.items)
	}
	if r.Effect != nil {
		c.checkRef("effect", r.Effect.ID, c.idx.effects)
	}
}

func (c *schemaChecker) checkCost(cost *types.POICost) {
	if cost == nil {
		return
	}
	for _, it := range cost.Items {
		c.checkRef("item", it.ID, c.idx.items)
	}
}

func (c *schemaChecker) checkNode(nodeID string, n types.POIStep, localNPCs map[string]bool) {
	old := c.context
	c.context = old + "/nodes." + nodeID
	defer func() { c.context = old }()

	switch n.Type {
	case types.POIStepCheck, types.POIStepPassiveCheck:
		c.checkSkill(n.Skill)
	case types.POIStepMonster:
		c.checkRef("monster", n.MonsterID, c.idx.monsters)
	case types.POIStepNPCInteraction:
		for _, id := range n.NPCIDs {
			if id == "" {
				continue
			}
			if !localNPCs[id] && !c.idx.npcs[id] {
				c.warnf("npc %q does not exist — needs to be created", id)
			}
		}
	}
	c.checkRequirements(n.Requirements)
	for i, ch := range n.Choices {
		save := c.context
		c.context = save + fmt.Sprintf(".choices[%d]", i)
		c.checkRequirements(ch.Requirements)
		c.context = save
	}
	c.checkLootTable(n.LootTable)
	c.checkReward(n.Reward)
	c.checkCost(n.Cost)
	if n.Effect != nil {
		c.checkRef("effect", n.Effect.ID, c.idx.effects)
	}
}

func (c *schemaChecker) checkPOI(p types.POIData) {
	c.checkRef("location", p.ParentEnvironment, c.idx.locations)
	c.checkSkill(p.Discovery.Skill)
	c.checkRequirements(p.Requirements)
	for _, id := range p.NPCIDs {
		c.checkNPCRef(id)
	}
	for id, n := range p.Nodes {
		// POIs no longer embed NPCs — the only NPC source is the global index.
		c.checkNode(id, n, nil)
	}
}

func (c *schemaChecker) checkEncounter(e types.EncounterData) {
	for _, loc := range e.ValidLocations {
		// for `building` trigger, IDs are buildings (not currently indexed); skip
		if e.Trigger == types.EncounterTriggerLocation || e.Trigger == types.EncounterTriggerTravel {
			c.checkRef("location", loc, c.idx.locations)
		}
	}
	c.checkRequirements(e.Requirements)
	local := map[string]bool{}
	for _, n := range e.NPCs {
		if n.ID != "" {
			local[n.ID] = true
		}
	}
	for id, n := range e.Nodes {
		c.checkNode(id, n, local)
	}
}

func (c *schemaChecker) checkQuest(q types.QuestData) {
	c.checkRequirements(q.Requirements)
	for _, pre := range q.Prerequisites {
		c.checkRef("quest", pre, c.idx.quests)
	}
	if q.StartCondition.Type == "talk" {
		c.checkNPCRef(q.StartCondition.Target)
	}
	if q.StartCondition.Location != "" {
		c.checkRef("location", q.StartCondition.Location, c.idx.locations)
	}
	for si, st := range q.Stages {
		save := c.context
		c.context = save + fmt.Sprintf("/stages[%d]", si)
		if st.UnlocksPOI != "" {
			c.checkRef("poi", st.UnlocksPOI, c.idx.pois)
		}
		c.checkReward(st.Rewards)
		for oi, o := range st.Objectives {
			oc := c.context
			c.context = oc + fmt.Sprintf(".objectives[%d]", oi)
			switch o.Type {
			case types.ObjectiveTalk:
				c.checkNPCRef(o.Target)
			case types.ObjectiveFetch:
				c.checkRef("item", o.Target, c.idx.items)
			case types.ObjectiveExplore:
				if !c.idx.locations[o.Target] && !c.idx.pois[o.Target] {
					c.errf("unknown location/poi: %q", o.Target)
				}
			case types.ObjectiveSlay:
				c.checkRef("monster", o.Target, c.idx.monsters)
			case types.ObjectiveCheck:
				c.checkSkill(o.Skill)
			default:
				c.errf("unknown objective type %q", o.Type)
			}
			c.context = oc
		}
		c.context = save
	}
}

// SchemaCheckResult is what ValidateSchema returns. Shape errors are decode
// failures; ref errors are unresolved item/monster/location/effect/quest/poi
// IDs (hard failures); ref warnings are unresolved NPC refs (authoring TODOs
// — the NPC file needs to be created but the schema is otherwise valid).
type SchemaCheckResult struct {
	OKFiles     []string `json:"ok_files"`
	ShapeErrors []string `json:"shape_errors"`
	RefErrors   []string `json:"ref_errors"`
	RefWarnings []string `json:"ref_warnings"`
}

// ValidateSchema runs the two-pass validator with the given source dirs.
// Pass an empty SchemaDirs{} to use defaults.
func ValidateSchema(dirs SchemaDirs) (*SchemaCheckResult, error) {
	if dirs.POIs == "" {
		dirs = DefaultSchemaDirs()
	}
	res := &SchemaCheckResult{}

	type loaded struct {
		path string
		kind string // "POI", "ENC", "Q"
		poi  *types.POIData
		enc  *types.EncounterData
		q    *types.QuestData
	}
	var files []loaded

	for _, p := range schemaWalkJSON(dirs.POIs) {
		var v types.POIData
		if err := schemaDecodeStrict(p, &v); err != nil {
			res.ShapeErrors = append(res.ShapeErrors, fmt.Sprintf("FAIL POI %s\n  %v", p, err))
			continue
		}
		res.OKFiles = append(res.OKFiles, fmt.Sprintf("ok   POI %s", p))
		files = append(files, loaded{p, "POI", &v, nil, nil})
	}
	for _, p := range schemaWalkJSON(dirs.Encounters) {
		var v types.EncounterData
		if err := schemaDecodeStrict(p, &v); err != nil {
			res.ShapeErrors = append(res.ShapeErrors, fmt.Sprintf("FAIL ENC %s\n  %v", p, err))
			continue
		}
		res.OKFiles = append(res.OKFiles, fmt.Sprintf("ok   ENC %s", p))
		files = append(files, loaded{p, "ENC", nil, &v, nil})
	}
	for _, p := range schemaWalkJSON(dirs.Quests) {
		var v types.QuestData
		if err := schemaDecodeStrict(p, &v); err != nil {
			res.ShapeErrors = append(res.ShapeErrors, fmt.Sprintf("FAIL Q   %s\n  %v", p, err))
			continue
		}
		res.OKFiles = append(res.OKFiles, fmt.Sprintf("ok   Q   %s", p))
		// Template carries placeholder IDs by design; skip ref check for it.
		if filepath.Base(p) == "template.json" {
			continue
		}
		files = append(files, loaded{p, "Q", nil, nil, &v})
	}

	if len(res.ShapeErrors) > 0 {
		return res, nil
	}

	idx := buildSchemaRefIndex(dirs)

	// Check external NPC home refs and filename convention.
	// Lenient decode (see buildSchemaRefIndex comment): legacy city NPC
	// files carry pre-refactor fields we don't want to flag here.
	for _, p := range schemaWalkJSON(dirs.NPCs) {
		b, err := os.ReadFile(p)
		if err != nil {
			res.ShapeErrors = append(res.ShapeErrors, fmt.Sprintf("FAIL NPC %s\n  %v", p, err))
			continue
		}
		var n types.NPCData
		if err := json.Unmarshal(b, &n); err != nil {
			res.ShapeErrors = append(res.ShapeErrors, fmt.Sprintf("FAIL NPC %s\n  %v", p, err))
			continue
		}
		var npcErrs []string
		ctx := fmt.Sprintf("[%s]", p)
		if n.PrimaryHome == "" {
			npcErrs = append(npcErrs, fmt.Sprintf("%s external NPC missing primary_home", ctx))
		} else if !idx.locations[n.PrimaryHome] && !idx.pois[n.PrimaryHome] {
			npcErrs = append(npcErrs, fmt.Sprintf("%s unknown primary_home: %q", ctx, n.PrimaryHome))
		}
		for _, sh := range n.SecondaryHomes {
			if !idx.locations[sh] && !idx.pois[sh] {
				npcErrs = append(npcErrs, fmt.Sprintf("%s unknown secondary_home: %q", ctx, sh))
			}
		}
		// filename convention: <NPCs>/<primary_home>/<id>.json
		rel, err := filepath.Rel(dirs.NPCs, p)
		if err == nil {
			parts := strings.Split(filepath.ToSlash(rel), "/")
			if len(parts) >= 2 && n.PrimaryHome != "" && parts[0] != n.PrimaryHome {
				npcErrs = append(npcErrs, fmt.Sprintf("%s filename dir %q does not match primary_home %q", ctx, parts[0], n.PrimaryHome))
			}
			if n.ID != "" && len(parts) >= 1 {
				base := strings.TrimSuffix(parts[len(parts)-1], ".json")
				if base != n.ID {
					npcErrs = append(npcErrs, fmt.Sprintf("%s filename %q does not match id %q", ctx, base, n.ID))
				}
			}
		}
		res.RefErrors = append(res.RefErrors, npcErrs...)
	}

	for _, f := range files {
		c := &schemaChecker{idx: idx, path: f.path, context: ""}
		switch f.kind {
		case "POI":
			c.checkPOI(*f.poi)
		case "ENC":
			c.checkEncounter(*f.enc)
		case "Q":
			c.checkQuest(*f.q)
		}
		if len(c.errs) > 0 {
			sort.Strings(c.errs)
			var b strings.Builder
			fmt.Fprintf(&b, "REF %s", f.path)
			for _, e := range c.errs {
				b.WriteString("\n  ")
				b.WriteString(e)
			}
			res.RefErrors = append(res.RefErrors, b.String())
		}
		if len(c.warns) > 0 {
			sort.Strings(c.warns)
			var b strings.Builder
			fmt.Fprintf(&b, "WARN %s", f.path)
			for _, w := range c.warns {
				b.WriteString("\n  ")
				b.WriteString(w)
			}
			res.RefWarnings = append(res.RefWarnings, b.String())
		}
	}

	sort.Strings(res.OKFiles)
	return res, nil
}

// ValidateSchemaIssues runs ValidateSchema and adapts the output to the
// package's standard []Issue shape so it can plug into ValidateAll().
func ValidateSchemaIssues() ([]Issue, error) {
	res, err := ValidateSchema(SchemaDirs{})
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, e := range res.ShapeErrors {
		issues = append(issues, Issue{Type: "error", Category: "schema", Message: e})
	}
	for _, e := range res.RefErrors {
		issues = append(issues, Issue{Type: "error", Category: "schema", Message: e})
	}
	for _, w := range res.RefWarnings {
		issues = append(issues, Issue{Type: "warning", Category: "schema", Message: w})
	}
	return issues, nil
}
