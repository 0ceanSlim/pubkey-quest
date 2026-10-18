package maintenance

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// A deploy's notice is picked up with its deadline, acknowledged with the
// online count, and cleared when the file goes away.
func TestNoticeLifecycle(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(noticePath, []byte(`{"in_seconds": 120, "version": "v9"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	poll(func() int { return 3 })

	n := Current()
	if n == nil || n.Version != "v9" {
		t.Fatalf("notice not picked up: %+v", n)
	}
	if left := n.At - time.Now().Unix(); left < 110 || left > 121 {
		t.Fatalf("deadline %ds away, want ~120", left)
	}
	var ack struct {
		Online int `json:"online"`
	}
	b, err := os.ReadFile(ackPath)
	if err != nil || json.Unmarshal(b, &ack) != nil || ack.Online != 3 {
		t.Fatalf("ack = %s (%v), want online 3", b, err)
	}

	// Re-reading the same file keeps the same deadline.
	at := n.At
	poll(func() int { return 0 })
	if got := Current(); got == nil || got.At != at {
		t.Fatalf("deadline moved: %+v", got)
	}

	os.Remove(noticePath)
	poll(func() int { return 0 })
	if Current() != nil {
		t.Fatal("notice should clear when the file is removed")
	}
}

func TestUpdateAvailable(t *testing.T) {
	mu.Lock()
	latest = "v0.1.20"
	mu.Unlock()
	if got := UpdateAvailable("v0.1.19"); got != "v0.1.20" {
		t.Fatalf("got %q", got)
	}
	if got := UpdateAvailable("v0.1.20"); got != "" {
		t.Fatalf("same version should report nothing, got %q", got)
	}
}
