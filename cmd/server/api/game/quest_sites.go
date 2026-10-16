package game

import (
	"encoding/json"
	"net/http"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// ─── Quest sites ─────────────────────────────────────────────────────────────
//
// A quest site is a place that exists only for a quest (types.QuestSite): the
// sewers behind the Salty Anchor's cellar wall. Unlike a POI it is never
// discovered or saved. Its entrance shows at its spot while the player is on a
// stage that names it, and inside it plays through the shared node walker, so
// its waypoints, checks and fights advance that stage like anything else.

// openSite is a quest site whose entrance is where the player stands.
type openSite struct {
	questID string
	siteID  string
	site    types.QuestSite
}

// sitesHere lists the quest sites open to the player at their current spot.
func sitesHere(save *types.SaveFile) []openSite {
	var out []openSite
	for _, qp := range save.QuestsActive {
		qd, err := serverdb.GetQuestByID(qp.QuestID)
		if err != nil || qd == nil || qp.Stage >= len(qd.Stages) {
			continue
		}
		id := qd.Stages[qp.Stage].Site
		site, ok := qd.Sites[id]
		if id == "" || !ok || !siteHere(site, save) {
			continue
		}
		out = append(out, openSite{questID: qd.ID, siteID: id, site: site})
	}
	return out
}

// siteHere reports whether the player stands at a site's entrance. Empty
// district/building/room in the site match anything.
func siteHere(s types.QuestSite, save *types.SaveFile) bool {
	return s.Location == save.Location &&
		(s.District == "" || s.District == save.District) &&
		(s.Building == "" || s.Building == save.Building) &&
		(s.Room == "" || s.Room == save.Room)
}

// QuestSitesHandler lists the quest-site entrances where the player stands.
// GET /api/quest/sites?npub&save_id
func QuestSitesHandler(w http.ResponseWriter, r *http.Request) {
	npub, saveID := r.URL.Query().Get("npub"), r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		writePOIError(w, http.StatusBadRequest, "missing npub or save_id")
		return
	}
	sess, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		writePOIError(w, http.StatusNotFound, "session not found")
		return
	}
	out := make([]map[string]any, 0)
	for _, s := range sitesHere(&sess.SaveData) {
		label := s.site.Label
		if label == "" {
			label = s.site.Name
		}
		out = append(out, map[string]any{
			"quest_id": s.questID, "site_id": s.siteID,
			"name": s.site.Name, "label": label, "description": s.site.Description,
		})
	}
	writePOIJSON(w, http.StatusOK, map[string]any{"success": true, "data": out})
}

// QuestSiteEnterHandler begins a walk of a quest site the player stands at.
// Advances then go through /api/poi/advance like any walk.
// POST /api/quest/site/enter {npub, save_id, quest_id, site_id}
func QuestSiteEnterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writePOIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Npub    string `json:"npub"`
		SaveID  string `json:"save_id"`
		QuestID string `json:"quest_id"`
		SiteID  string `json:"site_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Npub == "" || req.SaveID == "" {
		writePOIError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sess, err := session.GetSessionManager().GetSession(req.Npub, req.SaveID)
	if err != nil {
		writePOIError(w, http.StatusNotFound, "session not found")
		return
	}
	sess.Lock()
	defer sess.Unlock()
	applyPartySyncs(sess)
	if sess.ActiveCombat != nil || sess.ActivePOI != nil || sess.PendingEncounter != nil {
		writePOIError(w, http.StatusConflict, "You're busy.")
		return
	}
	var found *openSite
	for _, s := range sitesHere(&sess.SaveData) {
		if s.questID == req.QuestID && s.siteID == req.SiteID {
			found = &s
			break
		}
	}
	if found == nil {
		writePOIError(w, http.StatusBadRequest, "There's no way in here.")
		return
	}

	sess.ActivePOI = &poi.Session{
		POIID:       "quest:" + found.questID + ":" + found.siteID,
		Title:       found.site.Name,
		Nodes:       found.site.Nodes,
		CurrentNode: found.site.StartNode,
	}
	data := map[string]any{}
	res, err := stepPOI(sess, found.site.StartNode, data)
	if err != nil {
		sess.ActivePOI = nil
		writePOIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	data["step"] = res
	writePOIJSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}
