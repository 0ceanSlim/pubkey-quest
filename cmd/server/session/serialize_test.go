package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testNpub = "npub1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqshp52w2"

// One character's requests never overlap (two "end the fight" calls once paid
// the loot twice); different characters' do, and a request naming two slots
// is refused.
func TestSerializeRequests(t *testing.T) {
	var inside, most atomic.Int32
	h := SerializeRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inside.Add(1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		inside.Add(-1)
	}))
	run := func(saveIDs ...string) int32 {
		inside.Store(0)
		most.Store(0)
		var wg sync.WaitGroup
		for _, id := range saveIDs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				body := `{"npub":"` + testNpub + `","save_id":"` + id + `"}`
				req := httptest.NewRequest(http.MethodPost, "/api/combat/end", strings.NewReader(body))
				req.Header.Set("Content-Type", "text/plain") // the body is read whatever it claims to be
				h.ServeHTTP(httptest.NewRecorder(), req)
			}()
		}
		wg.Wait()
		return most.Load()
	}
	if got := run("save_1", "save_1", "save_1", "save_1", "save_1", "save_1"); got != 1 {
		t.Errorf("one character: %d requests at once", got)
	}
	if got := run("save_1", "save_2", "save_3", "save_4"); got < 2 {
		t.Errorf("four characters ran one at a time (most at once %d)", got)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/game/action?save_id=save_1",
		strings.NewReader(`{"npub":"`+testNpub+`","save_id":"save_2"}`)))
	if w.Code != http.StatusBadRequest {
		t.Errorf("two slots in one request: status %d", w.Code)
	}
	if n := len(slotLocks.m); n != 0 {
		t.Errorf("%d slot locks left behind", n)
	}
}
