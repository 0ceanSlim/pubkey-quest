package poi_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/types"
)

// NextsFor derives the anti-skip allowlist from a resolved step. These cases
// pin the rule the /advance guard relies on: choices expose every offered
// branch, a continue node exposes its single Next, and terminal / combat nodes
// expose nothing (the client can't advance past them itself).

func TestNextsForChoice(t *testing.T) {
	res := poi.Resolve(
		types.POIStep{Type: types.POIStepChoice, Choices: []types.POIChoice{
			{Label: "Left", Next: "a"},
			{Label: "Right", Next: "b"},
		}},
		"c1", &types.SaveFile{}, poi.Deps{Ctx: fakeCtx{}, Rng: rng()})

	got := poi.NextsFor(res)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("choice nexts = %v, want [a b]", got)
	}
}

func TestNextsForContinue(t *testing.T) {
	res := poi.Resolve(
		types.POIStep{Type: types.POIStepNarrative, Next: "n2"},
		"n1", &types.SaveFile{}, poi.Deps{Ctx: fakeCtx{}, Rng: rng()})

	got := poi.NextsFor(res)
	if len(got) != 1 || got[0] != "n2" {
		t.Errorf("narrative nexts = %v, want [n2]", got)
	}
}

func TestNextsForTerminalAndCombat(t *testing.T) {
	term := poi.Resolve(
		types.POIStep{Type: types.POIStepExit, Text: "You leave."},
		"ex", &types.SaveFile{}, poi.Deps{Ctx: fakeCtx{}, Rng: rng()})
	if n := poi.NextsFor(term); n != nil {
		t.Errorf("terminal nexts = %v, want nil", n)
	}

	// A monster node has a Next (the resume node), but the client must NOT be
	// able to advance to it — combat drives that. NextsFor must withhold it.
	mon := poi.Resolve(
		types.POIStep{Type: types.POIStepMonster, MonsterID: "wolf", Next: "after"},
		"m", &types.SaveFile{}, poi.Deps{Ctx: fakeCtx{}, Rng: rng()})
	if n := poi.NextsFor(mon); n != nil {
		t.Errorf("monster nexts = %v, want nil (combat resumes it)", n)
	}
}

func TestAllowsNext(t *testing.T) {
	s := &poi.Session{ValidNexts: []string{"a", "b"}}
	if !s.AllowsNext("a") {
		t.Error("AllowsNext should accept a listed node")
	}
	if s.AllowsNext("z") {
		t.Error("AllowsNext should reject an unlisted node (anti-skip)")
	}
	empty := &poi.Session{}
	if empty.AllowsNext("a") {
		t.Error("a session with no valid nexts should allow nothing")
	}
}

// The walker exposes the check result so the caller can record a passing check
// (advancing quest "check" objectives) without the walker importing the event
// recorder. These pin that CheckSkill/CheckSuccess are set on check nodes only.

func TestCheckResultExposed(t *testing.T) {
	node := types.POIStep{Type: types.POIStepCheck, Skill: "perception", DC: 5, SuccessNext: "win", FailureNext: "lose"}

	win := poi.Resolve(node, "ck", &types.SaveFile{},
		poi.Deps{Ctx: fakeCtx{skills: map[string]int{"perception": 20}}, Rng: rng()})
	if win.CheckSkill != "perception" || !win.CheckSuccess {
		t.Errorf("passing check: CheckSkill=%q CheckSuccess=%v, want perception/true", win.CheckSkill, win.CheckSuccess)
	}

	node.DC = 25
	lose := poi.Resolve(node, "ck", &types.SaveFile{},
		poi.Deps{Ctx: fakeCtx{skills: map[string]int{"perception": 4}}, Rng: rng()})
	if lose.CheckSkill != "perception" || lose.CheckSuccess {
		t.Errorf("failing check: CheckSkill=%q CheckSuccess=%v, want perception/false", lose.CheckSkill, lose.CheckSuccess)
	}
}

func TestCheckResultEmptyOnNonCheck(t *testing.T) {
	res := poi.Resolve(
		types.POIStep{Type: types.POIStepNarrative, Next: "n2"},
		"n1", &types.SaveFile{}, poi.Deps{Ctx: fakeCtx{}, Rng: rng()})
	if res.CheckSkill != "" || res.CheckSuccess {
		t.Errorf("narrative should carry no check result, got %q/%v", res.CheckSkill, res.CheckSuccess)
	}
}
