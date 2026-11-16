package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pubkey-quest/cmd/server/utils"
)

// Routes that let a request decide the game's outcome (start a fight with any
// monster, hand out items) exist only on a debug server.
func TestDebugOnlyRoutes(t *testing.T) {
	debugOnly := []string{"/api/combat/start", "/api/combat/debug/start", "/api/debug/state"}
	for _, debug := range []bool{false, true} {
		old := utils.AppConfig.Server.DebugMode
		utils.AppConfig.Server.DebugMode = debug
		mux := http.NewServeMux()
		RegisterRoutes(mux)
		utils.AppConfig.Server.DebugMode = old
		for _, path := range debugOnly {
			_, pattern := mux.Handler(httptest.NewRequest(http.MethodPost, path, nil))
			if registered := pattern == path; registered != debug {
				t.Errorf("debug %v: %s registered = %v (pattern %q)", debug, path, registered, pattern)
			}
		}
	}
}
