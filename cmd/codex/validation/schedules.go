package validation

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/types"
)

// ValidateSchedules checks town NPC schedules against their town (degradation
// of the world's life is quiet, so this is loud about it):
//   - every entry's place exists in the NPC's home town, its dialogue nodes
//     exist and its days are days of the week (errors);
//   - the everyday entries cover the whole day exactly once (error);
//   - nobody works inside a building while it's closed, and no building is
//     open with nobody working in it, hour by hour through the week (warnings).
//
// Buildings open "always" (vaults) are left out of the empty-building check,
// and an innkeeper dozing behind the counter with the night bell (asleep, but
// still renting rooms) counts as staff.
func ValidateSchedules() ([]Issue, error) {
	issues := []Issue{}
	towns, err := loadTowns()
	if err != nil {
		return nil, err
	}
	byTown := map[string][]schedNPC{}
	err = filepath.WalkDir("game-data/npcs", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var n types.NPCData
		if json.Unmarshal(raw, &n) != nil || len(n.Schedule) == 0 {
			return nil
		}
		town, ok := towns[n.PrimaryHome]
		if !ok {
			return nil
		}
		file := filepath.Base(path)
		issues = append(issues, checkSchedule(file, &n, town)...)
		byTown[n.PrimaryHome] = append(byTown[n.PrimaryHome], schedNPC{file, n})
		return nil
	})
	if err != nil {
		return nil, err
	}
	for id, npcs := range byTown {
		issues = append(issues, checkStaffing(towns[id], npcs)...)
	}
	return issues, nil
}

type schedNPC struct {
	file string
	data types.NPCData
}

type townHours struct {
	id        string
	file      string
	districts map[string]bool
	buildings map[string]schedBuilding
}

type schedBuilding struct {
	open, close interface{}
}

func loadTowns() (map[string]*townHours, error) {
	out := map[string]*townHours{}
	files, _ := filepath.Glob("game-data/locations/cities/*.json")
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var c struct {
			ID        string `json:"id"`
			Districts map[string]struct {
				ID        string `json:"id"`
				Buildings []struct {
					ID    string      `json:"id"`
					Open  interface{} `json:"open"`
					Close interface{} `json:"close"`
				} `json:"buildings"`
			} `json:"districts"`
		}
		if json.Unmarshal(raw, &c) != nil {
			continue
		}
		t := &townHours{id: c.ID, file: filepath.Base(f), districts: map[string]bool{}, buildings: map[string]schedBuilding{}}
		for _, d := range c.Districts {
			t.districts[d.ID] = true
			for _, b := range d.Buildings {
				t.buildings[b.ID] = schedBuilding{b.Open, b.Close}
			}
		}
		out[c.ID] = t
	}
	return out, nil
}

// openAt reports whether a building is open at a time of day (as the server's
// building package reads its hours).
func (b schedBuilding) openAt(t int) (open, always bool) {
	if s, ok := b.open.(string); ok && s == "always" {
		return true, true
	}
	o, ok := b.open.(float64)
	if !ok || o < 0 {
		return false, false
	}
	c, ok := b.close.(float64)
	if !ok {
		return t >= int(o), false
	}
	return types.InWindow(t, int(o), int(c)), false
}

func schedIssue(kind, file, msg string) Issue {
	return Issue{Type: kind, Category: "schedules", File: file, Field: "schedule", Message: msg}
}

func clock(m int) string { return fmt.Sprintf("%02d:%02d", m/60, m%60) }

func checkSchedule(file string, n *types.NPCData, town *townHours) []Issue {
	var issues []Issue
	days := map[string]bool{}
	for _, d := range types.WeekDays {
		days[strings.ToLower(d)] = true
	}
	cover := make([]int, 1440)
	for _, e := range n.Schedule {
		at := clock(e.Start)
		if _, ok := town.buildings[e.Location]; !ok && !town.districts[e.Location] {
			issues = append(issues, schedIssue("error", file, fmt.Sprintf("%s: %q isn't a building or district of %s", at, e.Location, town.id)))
		}
		for _, d := range e.DialogueOptions {
			if _, ok := n.Dialogue[d]; !ok {
				issues = append(issues, schedIssue("error", file, fmt.Sprintf("%s: dialogue node %q doesn't exist", at, d)))
			}
		}
		for _, d := range e.Days {
			if !days[strings.ToLower(d)] {
				issues = append(issues, schedIssue("error", file, fmt.Sprintf("%s: %q isn't a day of the week (%s)", at, d, strings.Join(types.WeekDays[:], ", "))))
			}
		}
		if len(e.Days) > 0 {
			continue
		}
		for t := 0; t < 1440; t++ {
			if types.InWindow(t, e.Start, e.End) || (e.Start == e.End) {
				cover[t]++
			}
		}
	}
	for t, n := range cover {
		if n != 1 {
			what := "has no entry"
			if n > 1 {
				what = "has overlapping entries"
			}
			issues = append(issues, schedIssue("error", file, fmt.Sprintf("the everyday schedule %s at %s", what, clock(t))))
			break
		}
	}
	return issues
}

// checkStaffing walks a town's week every half hour: who works where, and
// which open buildings stand empty.
func checkStaffing(town *townHours, npcs []schedNPC) []Issue {
	var issues []Issue
	empty := map[string][]string{}  // building -> "Firstday 03:30"
	closed := map[string][]string{} // npc file -> "Firstday 03:30 in x"
	for day := 1; day <= 7; day++ {
		for t := 0; t < 1440; t += 30 {
			staffed := map[string]bool{}
			for _, n := range npcs {
				s := npc.GetCurrentScheduleSlot(n.data.Schedule, day, t)
				if s == nil {
					continue
				}
				b, inBuilding := town.buildings[s.Location]
				if !inBuilding {
					continue
				}
				onDuty := s.State == "working" || (s.State == "sleeping" && len(s.AvailableActions) > 0)
				if !onDuty {
					continue
				}
				staffed[s.Location] = true
				if open, _ := b.openAt(t); !open && s.State == "working" {
					closed[n.file] = append(closed[n.file], fmt.Sprintf("%s %s in %s", types.Weekday(day), clock(t), s.Location))
				}
			}
			for id, b := range town.buildings {
				if open, always := b.openAt(t); open && !always && !staffed[id] {
					empty[id] = append(empty[id], fmt.Sprintf("%s %s", types.Weekday(day), clock(t)))
				}
			}
		}
	}
	for file, when := range closed {
		issues = append(issues, schedIssue("warning", file, fmt.Sprintf("works in a closed building %d times a week, e.g. %s", len(when), when[0])))
	}
	ids := make([]string, 0, len(empty))
	for id := range empty {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		issues = append(issues, Issue{Type: "warning", Category: "schedules", File: town.file, Field: id,
			Message: fmt.Sprintf("%s is open with nobody working %d half-hours a week, e.g. %s", id, len(empty[id]), empty[id][0])})
	}
	return issues
}
