package helpers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	originalDir string
	setupOnce   sync.Once
)

// SetupTestEnvironment changes to the project root directory where www/game.db exists.
// This should be called at the start of each test that requires database access.
func SetupTestEnvironment(t *testing.T) {
	t.Helper()
	setupOnce.Do(func() {
		var err error
		originalDir, err = os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// Find project root by looking for www/game.db
		// Start from the test file location and walk up
		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("Failed to get caller information")
		}

		// helpers.go is at tests/helpers/helpers.go
		// Project root is two directories up from tests/helpers
		helpersDir := filepath.Dir(filename)
		testsDir := filepath.Dir(helpersDir)
		projectRoot := filepath.Dir(testsDir)

		// Verify database exists
		dbPath := filepath.Join(projectRoot, "www", "game.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Skipf("Database not found at %s - skipping tests that require database", dbPath)
		}

		// Change to project root so db.InitDatabase() finds www/game.db
		if err := os.Chdir(projectRoot); err != nil {
			t.Fatalf("Failed to change to project root: %v", err)
		}
	})
}

// TestServer wraps httptest.Server with helper methods
type TestServer struct {
	*httptest.Server
	Mux *http.ServeMux
}

// NewTestServer creates a new test server with a fresh mux
func NewTestServer() *TestServer {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	return &TestServer{
		Server: server,
		Mux:    mux,
	}
}

// GET performs a GET request and returns the response
func (ts *TestServer) GET(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
	return resp
}

// POST performs a POST request with JSON body and returns the response
func (ts *TestServer) POST(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Failed to marshal POST body: %v", err)
	}
	resp, err := http.Post(ts.URL+path, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}
	return resp
}

// DELETE performs a DELETE request and returns the response
func (ts *TestServer) DELETE(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("DELETE", ts.URL+path, nil)
	if err != nil {
		t.Fatalf("Failed to create DELETE request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s failed: %v", path, err)
	}
	return resp
}

// ReadJSON reads the response body and unmarshals it into dest
func ReadJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v\nBody: %s", err, string(body))
	}
}

// AssertStatus checks that the response has the expected status code
func AssertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status %d, got %d. Body: %s", expected, resp.StatusCode, string(body))
	}
}

// AssertJSON checks that the response is valid JSON and returns it as a map
func AssertJSON(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	ReadJSON(t, resp, &result)
	return result
}

// AssertJSONArray checks that the response is a valid JSON array
func AssertJSONArray(t *testing.T, resp *http.Response) []interface{} {
	t.Helper()
	var result []interface{}
	ReadJSON(t, resp, &result)
	return result
}

// AssertSuccess checks that the response has success: true
func AssertSuccess(t *testing.T, result map[string]interface{}) {
	t.Helper()
	success, ok := result["success"].(bool)
	if !ok || !success {
		t.Errorf("Expected success: true, got: %v", result["success"])
	}
}

// AssertError checks that the response has success: false and an error message
func AssertError(t *testing.T, result map[string]interface{}) {
	t.Helper()
	success, ok := result["success"].(bool)
	if ok && success {
		t.Errorf("Expected success: false, got: true")
	}
	if _, ok := result["error"].(string); !ok {
		t.Errorf("Expected error message, got: %v", result["error"])
	}
}

// MockNpub is a test npub for use in tests
const MockNpub = "npub1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqshp52w2" // a well-formed npub (pubkey 0…01): handlers refuse malformed ones

// MockSaveID is a test save ID for use in tests
const MockSaveID = "save_test_12345"

// CreateMockSaveData creates a minimal save data structure for testing
func CreateMockSaveData() map[string]interface{} {
	return map[string]interface{}{
		"d":          "TestCharacter",
		"created_at": "2024-01-01T00:00:00Z",
		"race":       "Human",
		"class":      "Fighter",
		"background": "Soldier",
		"alignment":  "Lawful Good",
		"experience": 0,
		"hp":         10,
		"max_hp":     10,
		"mana":       0,
		"max_mana":   0,
		"fatigue":    0,
		"hunger":     2,
		"stats": map[string]interface{}{
			"strength":     16,
			"dexterity":    14,
			"constitution": 14,
			"intelligence": 10,
			"wisdom":       12,
			"charisma":     10,
		},
		"location":              "kingdom",
		"district":              "center",
		"building":              "",
		"current_day":           1,
		"time_of_day":           720,
		"inventory":             map[string]interface{}{},
		"vaults":                []interface{}{},
		"known_spells":          []string{},
		"spell_slots":           map[string]interface{}{},
		"locations_discovered":  []string{"kingdom"},
		"music_tracks_unlocked": []string{},
		"active_effects":        []interface{}{},
	}
}
