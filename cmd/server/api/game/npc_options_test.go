package game

import (
	"encoding/json"
	"testing"
)

func TestOptKeyRoundTrip(t *testing.T) {
	k := optKey("main_menu", 2)
	node, i, ok := parseOptKey(k)
	if !ok || node != "main_menu" || i != 2 {
		t.Fatalf("parseOptKey(%q) = %q, %d, %v", k, node, i, ok)
	}
	if _, _, ok := parseOptKey("goodbye"); ok {
		t.Fatal("a plain key isn't a labelled option")
	}
	// A node name with a colon still splits at the last one.
	if node, i, ok := parseOptKey("opt:a:b:7"); !ok || node != "a:b" || i != 7 {
		t.Fatalf("got %q, %d, %v", node, i, ok)
	}
}

func TestParseOption(t *testing.T) {
	var raw interface{}
	json.Unmarshal([]byte(`{
		"label": "[Charisma 12] I have rations to spare.",
		"next": "spare_rations",
		"requirements": [{"type": "stat", "id": "charisma", "min": 12}, {"type": "race", "values": ["Orc", "Half-Orc"]}],
		"cost": {"gold": 5, "items": [{"id": "rations", "quantity": 1}]}
	}`), &raw)
	o, ok := parseOption(raw)
	if !ok {
		t.Fatal("option not parsed")
	}
	if o.Next != "spare_rations" || len(o.Requirements) != 2 {
		t.Fatalf("got %+v", o)
	}
	if r := o.Requirements[0]; r.Type != "stat" || r.ID != "charisma" || r.Min != 12 {
		t.Fatalf("stat requirement: %+v", r)
	}
	if r := o.Requirements[1]; r.Type != "race" || len(r.Values) != 2 {
		t.Fatalf("race requirement: %+v", r)
	}
	if o.Cost.Gold != 5 || len(o.Cost.Items) != 1 || o.Cost.Items[0].ID != "rations" || o.Cost.Items[0].Quantity != 1 {
		t.Fatalf("cost: %+v", o.Cost)
	}
	if _, ok := parseOption("goodbye"); ok {
		t.Fatal("a plain key isn't a labelled option")
	}
	if _, ok := parseOption(map[string]interface{}{"label": "no next"}); ok {
		t.Fatal("an option without next is rejected")
	}
}
