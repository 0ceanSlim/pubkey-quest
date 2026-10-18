// Package maintenance warns players before the server restarts for an update,
// and (on release builds) notices when a newer release is out.
//
// A deploy announces a restart by writing data/restart-notice.json next to the
// running server, e.g. {"in_seconds": 120, "version": "v0.1.20"}. Only the host
// can write that file, so it needs no endpoint or auth. The server picks it up
// within a couple of seconds, shows every online player a countdown (through
// the social poll), and answers in data/restart-ack.json with how many players
// are online, so a deploy can skip the wait when nobody is.
package maintenance

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	noticePath = "data/restart-notice.json"
	ackPath    = "data/restart-ack.json"
	checkEvery = 2 * time.Second

	// releasesURL is where release builds look for a newer version.
	releasesURL       = "https://api.github.com/repos/0ceanSlim/pubkey-quest/releases/latest"
	releaseCheckEvery = time.Hour
)

// Notice is a restart the players are being warned about.
type Notice struct {
	At      int64  `json:"at"`                // unix seconds the restart is due
	Version string `json:"version,omitempty"` // what it updates to
	Reason  string `json:"reason,omitempty"`
}

var (
	mu      sync.RWMutex
	current *Notice
	stamp   time.Time // mtime of the notice file current came from
	latest  string    // newest release tag seen (release builds only)
)

// Current returns the pending restart, or nil.
func Current() *Notice {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return nil
	}
	n := *current
	return &n
}

// UpdateAvailable returns a newer release tag than running, or "".
func UpdateAvailable(running string) string {
	mu.RLock()
	defer mu.RUnlock()
	if latest == "" || latest == running {
		return ""
	}
	return latest
}

// Start watches for restart notices. online counts the players online now.
// A notice left over from before this process started is stale and removed.
func Start(online func() int) {
	if info, err := os.Stat(noticePath); err == nil {
		if due := readNotice(info.ModTime()); due == nil || time.Now().Unix() >= due.At {
			os.Remove(noticePath)
		}
	}
	os.Remove(ackPath)
	go func() {
		t := time.NewTicker(checkEvery)
		defer t.Stop()
		for range t.C {
			poll(online)
		}
	}()
}

func poll(online func() int) {
	info, err := os.Stat(noticePath)
	if err != nil {
		mu.Lock()
		cleared := current != nil
		current = nil
		mu.Unlock()
		if cleared {
			log.Printf("✅ Restart notice withdrawn")
		}
		return
	}
	mu.RLock()
	same := current != nil && info.ModTime().Equal(stamp)
	mu.RUnlock()
	if !same {
		n := readNotice(info.ModTime())
		if n == nil {
			return
		}
		mu.Lock()
		current, stamp = n, info.ModTime()
		mu.Unlock()
		log.Printf("⚠️ Restart notice: %s in %ds", orDefault(n.Version, "update"), n.At-time.Now().Unix())
	}
	writeAck(online())
}

// readNotice parses the notice file; the restart is due in_seconds after the
// file was written, so re-reading it never moves the deadline.
func readNotice(written time.Time) *Notice {
	b, err := os.ReadFile(noticePath)
	if err != nil {
		return nil
	}
	var f struct {
		InSeconds int    `json:"in_seconds"`
		Version   string `json:"version"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(b, &f); err != nil || f.InSeconds < 0 {
		log.Printf("⚠️ Ignoring %s: %v", noticePath, err)
		return nil
	}
	return &Notice{At: written.Add(time.Duration(f.InSeconds) * time.Second).Unix(), Version: f.Version, Reason: f.Reason}
}

func writeAck(online int) {
	b, _ := json.Marshal(map[string]any{"online": online, "at": time.Now().Unix()})
	tmp := ackPath + ".tmp"
	if err := os.MkdirAll(filepath.Dir(ackPath), 0o755); err != nil {
		return
	}
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, ackPath)
	}
}

// StartReleaseCheck looks for a newer release every hour (release builds only).
func StartReleaseCheck(running string) {
	go func() {
		for {
			if tag := fetchLatestTag(); tag != "" {
				mu.Lock()
				changed := tag != latest
				latest = tag
				mu.Unlock()
				if changed && tag != running {
					log.Printf("📦 New release %s is out (running %s) — promote it with \"Deploy: pubkey.quest\"", tag, running)
				}
			}
			time.Sleep(releaseCheckEvery)
		}
	}()
}

func fetchLatestTag() string {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, releasesURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(resp.Body).Decode(&rel) != nil {
		return ""
	}
	return strings.TrimSpace(rel.TagName)
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
