package report

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setup points the reporter at a temp dir and clears the rate limiter so each
// test starts clean. GitHub posting is a no-op because no token is configured.
func setup(t *testing.T) {
	t.Helper()
	reportsDir = t.TempDir()
	rateMu.Lock()
	rateHits = map[string][]time.Time{}
	rateMu.Unlock()
}

func post(t *testing.T, handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.7:12345"
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func logLines(t *testing.T, file string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(reportsDir, file))
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			lines = append(lines, sc.Text())
		}
	}
	return lines
}

func TestBugHandler_HappyPath(t *testing.T) {
	setup(t)

	rec := post(t, BugHandler, `{"npub":"npub1abc","save_id":"save_1","message":"  health bar overlaps  "}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("success = %v, want true", resp["success"])
	}

	lines := logLines(t, "bugs.jsonl")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want 1", len(lines))
	}
	var got bugRecord
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("decode log line: %v", err)
	}
	if got.Message != "health bar overlaps" {
		t.Errorf("message = %q, want trimmed %q", got.Message, "health bar overlaps")
	}
	if got.Type != "bug" || got.Npub != "npub1abc" {
		t.Errorf("unexpected record: %+v", got)
	}
}

func TestBugHandler_EmptyMessage(t *testing.T) {
	setup(t)

	rec := post(t, BugHandler, `{"npub":"npub1abc","save_id":"save_1","message":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if lines := logLines(t, "bugs.jsonl"); len(lines) != 0 {
		t.Errorf("expected no log lines, got %d", len(lines))
	}
}

func TestAccessHandler_HappyPath(t *testing.T) {
	setup(t)

	rec := post(t, AccessHandler, `{"npub":"npub1def","contact":"me@example.com","message":"please let me in"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	lines := logLines(t, "access-requests.jsonl")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want 1", len(lines))
	}
	var got accessRecord
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("decode log line: %v", err)
	}
	if got.Type != "access" || got.Npub != "npub1def" || got.Contact != "me@example.com" {
		t.Errorf("unexpected record: %+v", got)
	}
}

func TestAccessHandler_MissingNpub(t *testing.T) {
	setup(t)

	rec := post(t, AccessHandler, `{"contact":"me@example.com","message":"hi"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRateLimit(t *testing.T) {
	setup(t)

	// rateMax submissions succeed; the next is throttled (same IP, across types).
	for i := range rateMax {
		rec := post(t, BugHandler, `{"npub":"npub1abc","save_id":"save_1","message":"bug"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("submission %d: status = %d, want 200", i, rec.Code)
		}
	}
	rec := post(t, AccessHandler, `{"npub":"npub1abc","message":"one too many"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
}
