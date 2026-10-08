// Package api_test exercises the API handlers directly.
//
// These tests attach handlers to their own mux, which deliberately bypasses
// auth.RequireIdentity — the gate wraps the real server's mux in
// cmd/server/app.Start, not individual handlers. So nothing here can catch an
// unprotected route or an ownership regression; that is what the tests in
// cmd/server/auth cover. What these assert is handler behaviour given a request
// that has already passed the gate.
package api_test

import (
	"net/http"
	"testing"

	"pubkey-quest/cmd/server/api/game"
	"pubkey-quest/cmd/server/db"
	"pubkey-quest/tests/helpers"
)

func setupGameTestServer(t *testing.T) *helpers.TestServer {
	t.Helper()

	// Set up test environment (changes to project root for database access)
	helpers.SetupTestEnvironment(t)

	// Initialize database for tests
	if err := db.InitDatabase(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	ts := helpers.NewTestServer()
	ts.Mux.HandleFunc("/api/session/init", game.InitSessionHandler)
	ts.Mux.HandleFunc("/api/session/reload", game.ReloadSessionHandler)
	ts.Mux.HandleFunc("/api/session/state", game.GetSessionHandler)
	ts.Mux.HandleFunc("/api/session/update", game.UpdateSessionHandler)
	ts.Mux.HandleFunc("/api/session/save", game.SaveSessionHandler)
	ts.Mux.HandleFunc("/api/session/cleanup", game.CleanupSessionHandler)
	ts.Mux.HandleFunc("/api/game/action", game.GameActionHandler)
	ts.Mux.HandleFunc("/api/game/state", game.GetGameStateHandler)
	ts.Mux.HandleFunc("/api/shop/", game.ShopHandler)

	return ts
}

func TestInitSessionHandler(t *testing.T) {
	ts := setupGameTestServer(t)
	defer ts.Close()
	defer db.Close()

	t.Run("requires POST method", func(t *testing.T) {
		resp := ts.GET(t, "/api/session/init")
		helpers.AssertStatus(t, resp, http.StatusMethodNotAllowed)
	})

	t.Run("requires npub and save_id", func(t *testing.T) {
		body := map[string]interface{}{}
		resp := ts.POST(t, "/api/session/init", body)
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("fails for non-existent save", func(t *testing.T) {
		body := map[string]interface{}{
			"npub":    helpers.MockNpub,
			"save_id": "nonexistent_save",
		}
		resp := ts.POST(t, "/api/session/init", body)
		helpers.AssertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestReloadSessionHandler(t *testing.T) {
	ts := setupGameTestServer(t)
	defer ts.Close()
	defer db.Close()

	t.Run("requires POST method", func(t *testing.T) {
		resp := ts.GET(t, "/api/session/reload")
		helpers.AssertStatus(t, resp, http.StatusMethodNotAllowed)
	})

	t.Run("requires npub and save_id", func(t *testing.T) {
		body := map[string]interface{}{}
		resp := ts.POST(t, "/api/session/reload", body)
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestGetSessionHandler(t *testing.T) {
	ts := setupGameTestServer(t)
	defer ts.Close()
	defer db.Close()

	t.Run("requires GET method", func(t *testing.T) {
		resp := ts.POST(t, "/api/session/state", nil)
		helpers.AssertStatus(t, resp, http.StatusMethodNotAllowed)
	})

	t.Run("requires npub and save_id", func(t *testing.T) {
		resp := ts.GET(t, "/api/session/state")
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		resp := ts.GET(t, "/api/session/state?npub="+helpers.MockNpub+"&save_id=nonexistent")
		// Will try to load from disk, which should fail
		helpers.AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestUpdateSessionHandler(t *testing.T) {
	ts := setupGameTestServer(t)
	defer ts.Close()
	defer db.Close()

	t.Run("requires POST method", func(t *testing.T) {
		resp := ts.GET(t, "/api/session/update")
		helpers.AssertStatus(t, resp, http.StatusMethodNotAllowed)
	})

	t.Run("requires npub and save_id", func(t *testing.T) {
		body := map[string]interface{}{
			"save_data": helpers.CreateMockSaveData(),
		}
		resp := ts.POST(t, "/api/session/update", body)
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestSaveSessionHandler(t *testing.T) {
	ts := setupGameTestServer(t)
	defer ts.Close()
	defer db.Close()

	t.Run("requires POST method", func(t *testing.T) {
		resp := ts.GET(t, "/api/session/save")
		helpers.AssertStatus(t, resp, http.StatusMethodNotAllowed)
	})

	t.Run("requires npub and save_id", func(t *testing.T) {
		body := map[string]interface{}{}
		resp := ts.POST(t, "/api/session/save", body)
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("fails if session not in memory", func(t *testing.T) {
		body := map[string]interface{}{
			"npub":    helpers.MockNpub,
			"save_id": "nonexistent",
		}
		resp := ts.POST(t, "/api/session/save", body)
		helpers.AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestCleanupSessionHandler(t *testing.T) {
	ts := setupGameTestServer(t)
	defer ts.Close()
	defer db.Close()

	t.Run("requires DELETE method", func(t *testing.T) {
		resp := ts.GET(t, "/api/session/cleanup")
		helpers.AssertStatus(t, resp, http.StatusMethodNotAllowed)
	})

	t.Run("requires npub and save_id", func(t *testing.T) {
		resp := ts.DELETE(t, "/api/session/cleanup")
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("succeeds even for non-existent session", func(t *testing.T) {
		resp := ts.DELETE(t, "/api/session/cleanup?npub="+helpers.MockNpub+"&save_id=nonexistent")
		helpers.AssertStatus(t, resp, http.StatusOK)

		result := helpers.AssertJSON(t, resp)
		helpers.AssertSuccess(t, result)
	})
}

func TestGameActionHandler(t *testing.T) {
	ts := setupGameTestServer(t)
	defer ts.Close()
	defer db.Close()

	t.Run("requires POST method", func(t *testing.T) {
		resp := ts.GET(t, "/api/game/action")
		helpers.AssertStatus(t, resp, http.StatusMethodNotAllowed)
	})

	t.Run("requires npub and save_id", func(t *testing.T) {
		body := map[string]interface{}{
			"action": map[string]interface{}{
				"type":   "move",
				"params": map[string]interface{}{},
			},
		}
		resp := ts.POST(t, "/api/game/action", body)
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("fails for non-existent session", func(t *testing.T) {
		body := map[string]interface{}{
			"npub":    helpers.MockNpub,
			"save_id": "nonexistent",
			"action": map[string]interface{}{
				"type":   "move",
				"params": map[string]interface{}{},
			},
		}
		resp := ts.POST(t, "/api/game/action", body)
		helpers.AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestGetGameStateHandler(t *testing.T) {
	ts := setupGameTestServer(t)
	defer ts.Close()
	defer db.Close()

	t.Run("requires GET method", func(t *testing.T) {
		resp := ts.POST(t, "/api/game/state", nil)
		helpers.AssertStatus(t, resp, http.StatusMethodNotAllowed)
	})

	t.Run("requires npub and save_id", func(t *testing.T) {
		resp := ts.GET(t, "/api/game/state")
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestShopHandler(t *testing.T) {
	ts := setupGameTestServer(t)
	defer ts.Close()
	defer db.Close()

	t.Run("GET requires merchant ID", func(t *testing.T) {
		resp := ts.GET(t, "/api/shop/")
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("GET requires npub and save_id", func(t *testing.T) {
		resp := ts.GET(t, "/api/shop/some-merchant")
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("POST buy requires valid body", func(t *testing.T) {
		resp := ts.POST(t, "/api/shop/buy", "invalid")
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("POST sell requires valid body", func(t *testing.T) {
		resp := ts.POST(t, "/api/shop/sell", "invalid")
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})
}
