package session

import (
	"sync"
	"testing"
	"time"

	"pubkey-quest/types"
)

// A save that's slow to load (fetched from relays) doesn't hold up anyone
// else's session, and two loads of the same save end up with one session.
func TestSlowLoadDoesNotBlockOthers(t *testing.T) {
	t.Chdir(t.TempDir()) // no journals or stats here
	sm := NewSessionManager()
	npub := "npub1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqshp52w2"
	fast := func(string, string) (*types.SaveFile, error) { return &types.SaveFile{}, nil }
	if _, err := sm.LoadSession(npub, "save_1", fast, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	release, started := make(chan struct{}), make(chan struct{}, 2)
	slow := func(string, string) (*types.SaveFile, error) {
		started <- struct{}{}
		<-release
		return &types.SaveFile{}, nil
	}
	var wg sync.WaitGroup
	got := make([]*GameSession, 2)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], _ = sm.LoadSession(npub, "save_2", slow, nil, nil, nil)
		}()
	}

	<-started // a slow load is under way
	done := make(chan error, 1)
	go func() { _, err := sm.GetSession(npub, "save_1"); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("another player's session waited on a slow load")
	}

	close(release)
	wg.Wait()
	if got[0] == nil || got[0] != got[1] {
		t.Errorf("two loads of one save made two sessions: %p %p", got[0], got[1])
	}
}
