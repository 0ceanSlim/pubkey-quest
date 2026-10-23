package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"pubkey-quest/cmd/server/api/game"
	"pubkey-quest/tests/helpers"
)

// The Wait window's preview is the real wait run on a copy of the save: it
// must leave the save alone and match what waiting then actually does.
func TestWaitPreview_MatchesTheRealWait(t *testing.T) {
	ts := setupTutorialServer(t)
	ts.Mux.HandleFunc("/api/game/wait-preview", game.WaitPreviewHandler)
	saveID := newCharacter(t, ts, tutorialNpubB)

	s := liveSave(t, tutorialNpubB, saveID)
	s.HP = max(1, s.MaxHP/3)
	s.Fatigue = 4
	s.Hunger = 3
	day, minute := s.CurrentDay, s.TimeOfDay

	resp := ts.GET(t, "/api/game/wait-preview?npub="+tutorialNpubB+"&save_id="+saveID)
	helpers.AssertStatus(t, resp, http.StatusOK)
	var out struct {
		Success bool `json:"success"`
		Data    struct {
			HP      int             `json:"hp"`
			Fatigue int             `json:"fatigue"`
			Steps   []game.WaitStep `json:"steps"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || !out.Success {
		t.Fatalf("preview: %v %+v", err, out)
	}
	if len(out.Data.Steps) != 24 || out.Data.Steps[0].Minutes != 15 || out.Data.Steps[23].Minutes != 360 {
		t.Fatalf("want 24 steps of 15 minutes, got %d", len(out.Data.Steps))
	}
	if out.Data.Fatigue != 4 {
		t.Errorf("fatigue = %d, want 4", out.Data.Fatigue)
	}

	s = liveSave(t, tutorialNpubB, saveID)
	if s.CurrentDay != day || s.TimeOfDay != minute || s.HP != out.Data.HP {
		t.Fatalf("the preview changed the save: day %d→%d, time %d→%d", day, s.CurrentDay, minute, s.TimeOfDay)
	}

	// Four hours: the hunger tick and real healing both land inside it.
	want := out.Data.Steps[15]
	if want.Minutes != 240 {
		t.Fatalf("step 15 is %d minutes", want.Minutes)
	}
	if want.HP <= out.Data.HP {
		t.Errorf("preview shows no healing: %d → %d", out.Data.HP, want.HP)
	}
	if r := act(t, ts, tutorialNpubB, saveID, "wait", map[string]any{"minutes": 240}); r["success"] != true {
		t.Fatalf("wait: %v", r)
	}
	s = liveSave(t, tutorialNpubB, saveID)
	if s.HP != want.HP || s.Mana != want.Mana || s.Hunger != want.Hunger ||
		s.TimeOfDay != want.TimeOfDay || s.CurrentDay != want.Day {
		t.Errorf("preview %+v, but the wait gave hp %d mana %d hunger %d at day %d %d",
			want, s.HP, s.Mana, s.Hunger, s.CurrentDay, s.TimeOfDay)
	}
	if s.Fatigue != 4 {
		t.Errorf("waiting changed fatigue to %d", s.Fatigue)
	}
}
