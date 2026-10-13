package game

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// The Shadow Alley Den is a city POI: it sits in Goldenhaven's east district,
// is noticed while spending time there, and is entered from the street.
func TestCityPOIIsFoundAndEnteredInItsDistrict(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	den, err := serverdb.GetPOIByID("shadow-alley-den")
	if err != nil {
		t.Fatal(err)
	}
	street := &types.SaveFile{Location: "goldenhaven", District: "east"}
	if !poiHere(street, den) {
		t.Error("the den should be enterable from the east district's streets")
	}
	for name, s := range map[string]*types.SaveFile{
		"another district": {Location: "goldenhaven", District: "center"},
		"inside a building": {Location: "goldenhaven", District: "east", Building: "sailors_tavern"},
		"on the road":       {Location: "merchants-highway", District: "goldenhaven-west"},
	} {
		if poiHere(s, den) {
			t.Errorf("the den shouldn't be enterable from %s", name)
		}
	}

	// Spending time there eventually turns it up (20% a visit, once per visit).
	found := false
	for visit := 0; visit < 60 && !found; visit++ {
		sess := &session.GameSession{SaveData: types.SaveFile{Location: "goldenhaven", District: "east", TimeOfDay: 600},
			PlaceSince: 600 - 60, PlaceInCity: true}
		resp := &types.GameActionResponse{}
		maybeNoticeCityPOI(sess, resp)
		if !sess.PlaceSearched {
			t.Fatal("a visit should get its one look around")
		}
		found = alreadyDiscovered(&sess.SaveData, "shadow-alley-den")
		if found && resp.Data["poi_discovered"] == nil {
			t.Error("a discovery should be announced")
		}
	}
	if !found {
		t.Error("60 visits never noticed the den")
	}

	// A road POI is only entered from its road.
	tower, err := serverdb.GetPOIByID("abandoned-watchtower")
	if err != nil {
		t.Fatal(err)
	}
	if poiHere(street, tower) || !poiHere(&types.SaveFile{Location: tower.ParentEnvironment}, tower) {
		t.Error("a road POI belongs to its road")
	}
}
