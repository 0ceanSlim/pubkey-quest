package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pubkey-quest/cmd/server/utils"
)

func TestLaunchGate(t *testing.T) {
	prev := utils.AppConfig.Launch
	defer func() { utils.AppConfig.Launch = prev }()
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	gate := LaunchGate(ok)
	code := func(path string) int {
		rec := httptest.NewRecorder()
		gate.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec.Code
	}

	utils.AppConfig.Launch.Closed = false
	if code("/game") != 200 {
		t.Fatal("open server should pass everything")
	}

	utils.AppConfig.Launch = utils.LaunchConfig{Closed: true, TestURL: "https://test.pubkey.quest"}
	for path, want := range map[string]int{
		"/":                503,
		"/game":            503,
		"/api/game/action": 503,
		"/?signin=1":       200,
		"/api/auth/login":  200,
		"/res/img/x.png":   200,
		"/dist/game.js":    200,
	} {
		if got := code(path); got != want {
			t.Errorf("closed %s: %d, want %d", path, got, want)
		}
	}
}
