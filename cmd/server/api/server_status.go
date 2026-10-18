package api

import (
	"encoding/json"
	"net/http"

	"pubkey-quest/cmd/server/maintenance"
	"pubkey-quest/cmd/server/utils"
)

// ServerStatusHandler reports the running version, a newer release when one
// is out (release builds check), and a pending restart warning.
func ServerStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":          true,
		"version":          utils.AppVersion,
		"update_available": maintenance.UpdateAvailable(utils.AppVersion),
		"restart":          maintenance.Current(),
	})
}
