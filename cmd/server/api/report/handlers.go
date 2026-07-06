package report

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/utils"
)

// ---------------------------------------------------------------------------
// Bug reports — submitted from inside the game (player is authenticated), so we
// enrich the report with authoritative context from the in-memory session.
// ---------------------------------------------------------------------------

type bugRequest struct {
	Npub    string `json:"npub"`
	SaveID  string `json:"save_id"`
	Message string `json:"message"`
}

type bugRecord struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Npub      string `json:"npub"`
	SaveID    string `json:"save_id,omitempty"`
	Location  string `json:"location,omitempty"`
	Room      string `json:"room,omitempty"`
	Level     int    `json:"level,omitempty"`
	Race      string `json:"race,omitempty"`
	Class     string `json:"class,omitempty"`
	Message   string `json:"message"`
}

// BugHandler godoc
// @Summary      Submit a bug report
// @Description  Appends the report to data/reports/bugs.jsonl and, when configured, mirrors it to a pinned GitHub issue. No GitHub account required for the player.
// @Tags         Report
// @Accept       json
// @Produce      json
// @Param        request  body      bugRequest  true  "npub, save_id, message"
// @Success      200      {object}  map[string]any
// @Failure      400      {object}  map[string]any  "Empty message"
// @Failure      429      {object}  map[string]any  "Rate limited"
// @Router       /report/bug [post]
func BugHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"success": false, "error": "Method not allowed"})
		return
	}
	if !allow(clientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"success": false, "error": "Too many reports — please wait a moment."})
		return
	}

	var req bugRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Invalid request body"})
		return
	}

	msg := sanitize(req.Message)
	if msg == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Please describe the bug before submitting."})
		return
	}

	rec := bugRecord{
		Type:      "bug",
		Timestamp: nowFunc().UTC().Format(time.RFC3339),
		Npub:      req.Npub,
		SaveID:    req.SaveID,
		Message:   msg,
	}
	enrichBug(&rec, req.Npub, req.SaveID)

	if err := appendLog("bugs.jsonl", rec); err != nil {
		log.Printf("❌ Reporter: failed to log bug report: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "Failed to save report."})
		return
	}

	// Best-effort GitHub mirror — the local log already captured it.
	if err := postIssueComment(utils.AppConfig.Report.BugIssueNumber, formatBugComment(rec)); err != nil {
		log.Printf("⚠️ Reporter: bug logged but GitHub post failed: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Bug report received — thank you!"})
}

// enrichBug fills in authoritative context from the live session, if present.
// A missing session (player not loaded) just leaves the fields empty.
func enrichBug(rec *bugRecord, npub, saveID string) {
	if npub == "" || saveID == "" {
		return
	}
	sess, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		return
	}
	save := &sess.SaveData
	rec.Location = save.Location
	if save.Building != "" {
		rec.Location = save.Location + "/" + save.Building
	}
	rec.Room = save.Room
	rec.Race = save.Race
	rec.Class = save.Class

	if adv, err := character.LoadAdvancement(serverdb.GetDB()); err == nil {
		rec.Level = character.GetLevelFromXP(save.Experience, adv)
	}
}

func formatBugComment(rec bugRecord) string {
	ctx := fmt.Sprintf("%s @ %s", shortNpub(rec.Npub), rec.Location)
	if rec.Room != "" {
		ctx += "/" + rec.Room
	}
	if rec.Level > 0 {
		ctx += fmt.Sprintf(", Lvl %d", rec.Level)
	}
	if rec.Class != "" {
		ctx += fmt.Sprintf(" %s %s", rec.Race, rec.Class)
	}
	if rec.SaveID != "" {
		ctx += ", " + rec.SaveID
	}
	return fmt.Sprintf("### 🐛 Bug report\n\n**%s**\n_%s UTC_\n\n%s", ctx, rec.Timestamp, rec.Message)
}

// ---------------------------------------------------------------------------
// Access requests — submitted from the login/whitelist-denied screen. The
// player is by definition NOT authenticated or whitelisted, so this endpoint is
// open (rate-limited). The npub is captured automatically at denial time.
// ---------------------------------------------------------------------------

type accessRequest struct {
	Npub    string `json:"npub"`
	Contact string `json:"contact"`
	Message string `json:"message"`
}

type accessRecord struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Npub      string `json:"npub"`
	Contact   string `json:"contact,omitempty"`
	Message   string `json:"message,omitempty"`
}

// AccessHandler godoc
// @Summary      Request test-server access
// @Description  Appends an access request to data/reports/access-requests.jsonl and, when configured, mirrors it to a pinned GitHub issue.
// @Tags         Report
// @Accept       json
// @Produce      json
// @Param        request  body      accessRequest  true  "npub, optional contact and message"
// @Success      200      {object}  map[string]any
// @Failure      400      {object}  map[string]any  "Missing npub"
// @Failure      429      {object}  map[string]any  "Rate limited"
// @Router       /report/access [post]
func AccessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"success": false, "error": "Method not allowed"})
		return
	}
	if !allow(clientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"success": false, "error": "Too many requests — please wait a moment."})
		return
	}

	var req accessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Invalid request body"})
		return
	}

	npub := sanitize(req.Npub)
	if npub == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "Missing npub."})
		return
	}

	rec := accessRecord{
		Type:      "access",
		Timestamp: nowFunc().UTC().Format(time.RFC3339),
		Npub:      npub,
		Contact:   sanitize(req.Contact),
		Message:   sanitize(req.Message),
	}

	if err := appendLog("access-requests.jsonl", rec); err != nil {
		log.Printf("❌ Reporter: failed to log access request: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "Failed to save request."})
		return
	}

	if err := postIssueComment(utils.AppConfig.Report.AccessIssueNumber, formatAccessComment(rec)); err != nil {
		log.Printf("⚠️ Reporter: access request logged but GitHub post failed: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Access request received — you'll be notified once approved."})
}

func formatAccessComment(rec accessRecord) string {
	body := fmt.Sprintf("### 🔑 Access request\n\n**npub:** `%s`\n_%s UTC_\n", rec.Npub, rec.Timestamp)
	if rec.Contact != "" {
		body += fmt.Sprintf("\n**Contact:** %s\n", rec.Contact)
	}
	if rec.Message != "" {
		body += fmt.Sprintf("\n%s\n", rec.Message)
	}
	return body
}
