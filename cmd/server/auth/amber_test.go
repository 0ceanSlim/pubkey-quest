package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The callback page leaves Amber's answer under the game tab's code; the tab
// collects it once, and nothing answers for a code that's malformed.
func TestAmberResultRelay(t *testing.T) {
	h := &AuthHandler{}
	code := strings.Repeat("ab", 16)
	get := func(c string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.HandleAmberResult(w, httptest.NewRequest(http.MethodGet, "/api/auth/amber-result?code="+c, nil))
		return w
	}
	post := func(body string) int {
		w := httptest.NewRecorder()
		h.HandleAmberResult(w, httptest.NewRequest(http.MethodPost, "/api/auth/amber-result", strings.NewReader(body)))
		return w.Code
	}

	if w := get(code); w.Code != http.StatusNoContent {
		t.Fatalf("an answer before Amber gave one: %d", w.Code)
	}
	if c := post(`{"code":"` + code + `","event":"{\"sig\":\"x\"}"}`); c != http.StatusNoContent {
		t.Fatalf("posting the answer: %d", c)
	}
	if w := get(code); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `\"sig\"`) {
		t.Fatalf("collecting it: %d %s", w.Code, w.Body)
	}
	if w := get(code); w.Code != http.StatusNoContent {
		t.Errorf("collected twice: %d", w.Code)
	}

	for _, bad := range []string{
		`{"code":"short","event":"x"}`,
		`{"code":"` + strings.Repeat("AB", 16) + `","event":"x"}`, // codes are lowercase hex
		`{"code":"` + code + `"}`,                               // no answer
		`not json`,
	} {
		if c := post(bad); c != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", bad, c)
		}
	}
	if w := get("../x"); w.Code != http.StatusBadRequest {
		t.Errorf("a malformed code: %d", w.Code)
	}
}
