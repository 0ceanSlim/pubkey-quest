package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"pubkey-quest/cmd/server/api"
	"pubkey-quest/cmd/server/db"
	"pubkey-quest/tests/helpers"
)

func setupSavesTestServer(t *testing.T) *helpers.TestServer {
	t.Helper()

	// Set up test environment (changes to project root for database access)
	helpers.SetupTestEnvironment(t)

	// Initialize database for tests
	if err := db.InitDatabase(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	ts := helpers.NewTestServer()
	ts.Mux.HandleFunc("/api/saves/", api.SavesHandler)

	return ts
}

// cleanupTestSaves removes any test save files created during tests
func cleanupTestSaves(t *testing.T, npub string) {
	t.Helper()
	savesDir := filepath.Join(api.SavesDirectory, npub)
	os.RemoveAll(savesDir)
}

func TestSavesHandler_GetSaves(t *testing.T) {
	ts := setupSavesTestServer(t)
	defer ts.Close()
	defer db.Close()
	defer cleanupTestSaves(t, helpers.MockNpub)

	t.Run("returns empty array for new user", func(t *testing.T) {
		resp := ts.GET(t, "/api/saves/"+helpers.MockNpub)
		helpers.AssertStatus(t, resp, http.StatusOK)

		saves := helpers.AssertJSONArray(t, resp)
		if len(saves) != 0 {
			t.Errorf("Expected empty saves array, got %d saves", len(saves))
		}
	})

	t.Run("requires npub in URL", func(t *testing.T) {
		resp := ts.GET(t, "/api/saves/")
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestSavesHandler_CreateSave(t *testing.T) {
	ts := setupSavesTestServer(t)
	defer ts.Close()
	defer db.Close()
	defer cleanupTestSaves(t, helpers.MockNpub)

	t.Run("creates new save", func(t *testing.T) {
		saveData := helpers.CreateMockSaveData()
		resp := ts.POST(t, "/api/saves/"+helpers.MockNpub, saveData)
		helpers.AssertStatus(t, resp, http.StatusOK)

		result := helpers.AssertJSON(t, resp)
		helpers.AssertSuccess(t, result)

		if _, ok := result["save_id"].(string); !ok {
			t.Error("Expected save_id in response")
		}
	})

	t.Run("overwrites existing save with same ID", func(t *testing.T) {
		// First create a save
		saveData := helpers.CreateMockSaveData()
		resp := ts.POST(t, "/api/saves/"+helpers.MockNpub, saveData)
		helpers.AssertStatus(t, resp, http.StatusOK)

		result := helpers.AssertJSON(t, resp)
		saveID := result["save_id"].(string)

		// Now update it with the same ID
		saveData["id"] = saveID
		saveData["d"] = "UpdatedCharacter"
		resp2 := ts.POST(t, "/api/saves/"+helpers.MockNpub, saveData)
		helpers.AssertStatus(t, resp2, http.StatusOK)

		result2 := helpers.AssertJSON(t, resp2)
		helpers.AssertSuccess(t, result2)

		// Verify it's the same save ID
		if result2["save_id"] != saveID {
			t.Errorf("Expected same save_id, got %v", result2["save_id"])
		}
	})

	t.Run("rejects invalid save data", func(t *testing.T) {
		resp := ts.POST(t, "/api/saves/"+helpers.MockNpub, "invalid json")
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestSavesHandler_DeleteSave(t *testing.T) {
	ts := setupSavesTestServer(t)
	defer ts.Close()
	defer db.Close()
	defer cleanupTestSaves(t, helpers.MockNpub)

	t.Run("requires save ID", func(t *testing.T) {
		resp := ts.DELETE(t, "/api/saves/"+helpers.MockNpub)
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 404 for non-existent save", func(t *testing.T) {
		resp := ts.DELETE(t, "/api/saves/"+helpers.MockNpub+"/save_1")
		helpers.AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("deletes existing save", func(t *testing.T) {
		// First create a save
		saveData := helpers.CreateMockSaveData()
		createResp := ts.POST(t, "/api/saves/"+helpers.MockNpub, saveData)
		helpers.AssertStatus(t, createResp, http.StatusOK)

		result := helpers.AssertJSON(t, createResp)
		saveID := result["save_id"].(string)

		// Now delete it
		deleteResp := ts.DELETE(t, "/api/saves/"+helpers.MockNpub+"/"+saveID)
		helpers.AssertStatus(t, deleteResp, http.StatusOK)

		deleteResult := helpers.AssertJSON(t, deleteResp)
		helpers.AssertSuccess(t, deleteResult)

		// Verify it's deleted by trying to delete again
		deleteResp2 := ts.DELETE(t, "/api/saves/"+helpers.MockNpub+"/"+saveID)
		helpers.AssertStatus(t, deleteResp2, http.StatusNotFound)
	})
}

func TestSavesHandler_FullCycle(t *testing.T) {
	ts := setupSavesTestServer(t)
	defer ts.Close()
	defer db.Close()
	defer cleanupTestSaves(t, helpers.MockNpub)

	// 1. Start with no saves
	resp := ts.GET(t, "/api/saves/"+helpers.MockNpub)
	helpers.AssertStatus(t, resp, http.StatusOK)
	saves := helpers.AssertJSONArray(t, resp)
	if len(saves) != 0 {
		t.Fatalf("Expected 0 saves, got %d", len(saves))
	}

	// 2. Create a save
	saveData := helpers.CreateMockSaveData()
	resp = ts.POST(t, "/api/saves/"+helpers.MockNpub, saveData)
	helpers.AssertStatus(t, resp, http.StatusOK)
	result := helpers.AssertJSON(t, resp)
	saveID := result["save_id"].(string)

	// 3. Verify save exists
	resp = ts.GET(t, "/api/saves/"+helpers.MockNpub)
	helpers.AssertStatus(t, resp, http.StatusOK)
	saves = helpers.AssertJSONArray(t, resp)
	if len(saves) != 1 {
		t.Fatalf("Expected 1 save, got %d", len(saves))
	}

	// 4. Delete the save
	resp = ts.DELETE(t, "/api/saves/"+helpers.MockNpub+"/"+saveID)
	helpers.AssertStatus(t, resp, http.StatusOK)

	// 5. Verify save is deleted
	resp = ts.GET(t, "/api/saves/"+helpers.MockNpub)
	helpers.AssertStatus(t, resp, http.StatusOK)
	saves = helpers.AssertJSONArray(t, resp)
	if len(saves) != 0 {
		t.Fatalf("Expected 0 saves after delete, got %d", len(saves))
	}
}

// A save id or npub that isn't one the game makes never reaches the disk: they
// become file paths, and "../" once reached other players' files.
func TestSavesHandler_RefusesBadPaths(t *testing.T) {
	ts := setupSavesTestServer(t)
	defer ts.Close()
	defer db.Close()
	for _, target := range []string{
		"/api/saves/" + helpers.MockNpub + "/..%2F..%2Fledger%2Fx",
		"/api/saves/" + helpers.MockNpub + "/save_1.json",
		"/api/saves/npub1notreal/save_1",
	} {
		if resp := ts.DELETE(t, target); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("DELETE %s: status %d, want 400", target, resp.StatusCode)
		}
	}
}
