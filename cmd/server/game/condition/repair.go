package condition

import (
	"encoding/json"
	"math"
	"sync"

	"pubkey-quest/cmd/server/db"
)

// Repair (degradation §5). Field repair (a whetstone for edged weapons, a
// crafting kit for armour and blunt weapons) only brings gear back up to Worn;
// a smith makes it new, at a price that depends on the smith. The rules live in
// game-data/systems/condition.json ("repair", "materials").

// SmithDeal is one smith's terms.
type SmithDeal struct {
	Materials []string `json:"materials"`          // what they work: metal, wood, leather, hide
	Cost      float64  `json:"cost"`               // × the item's value × the share of wear mended
	Material  string   `json:"material,omitempty"` // an item they want 1 of per 25% of wear mended
	UpTo      string   `json:"up_to,omitempty"`    // "worn": rough work, only back to Worn
	Discount  *struct {
		Item    string `json:"item"`
		Percent int    `json:"percent"`
	} `json:"discount,omitempty"` // bring this and pay less (1 per repair)
}

// RepairRules is condition.json's "repair" block, with its "materials".
type RepairRules struct {
	FieldUpTo float64 `json:"field_up_to"`
	Whetstone struct {
		Minutes     int      `json:"minutes"`
		DamageTypes []string `json:"damage_types"`
	} `json:"whetstone"`
	CraftingKit struct {
		Minutes int    `json:"minutes"`
		Skill   string `json:"skill"`
		DC      int    `json:"dc"`
	} `json:"crafting_kit"`
	Smiths    map[string]SmithDeal `json:"smiths"`
	materials map[string]string    // item id → material
}

var (
	repairOnce  sync.Once
	repairRules RepairRules
)

// Repair returns the repair rules (read once from the database).
func Repair() *RepairRules {
	repairOnce.Do(func() {
		repairRules = RepairRules{FieldUpTo: 0.74, materials: map[string]string{}}
		database := db.GetDB()
		if database == nil {
			return
		}
		var props string
		if database.QueryRow("SELECT properties FROM systems WHERE id = 'condition'").Scan(&props) != nil {
			return
		}
		var cfg struct {
			Repair    RepairRules         `json:"repair"`
			Materials map[string][]string `json:"materials"`
		}
		if json.Unmarshal([]byte(props), &cfg) != nil {
			return
		}
		repairRules = cfg.Repair
		if repairRules.FieldUpTo <= 0 {
			repairRules.FieldUpTo = 0.74
		}
		repairRules.materials = map[string]string{}
		for material, ids := range cfg.Materials {
			for _, id := range ids {
				repairRules.materials[id] = material
			}
		}
	})
	return &repairRules
}

// ResetRepairCache forgets the repair rules (tests).
func ResetRepairCache() { repairOnce = sync.Once{} }

// MaterialOf is what an item is made of (metal unless listed).
func MaterialOf(itemID string) string {
	if m, ok := Repair().materials[itemID]; ok {
		return m
	}
	return "metal"
}

// wearAt is the wear that leaves share of an item's durability.
func wearAt(max int, share float64) int {
	return int(math.Ceil(float64(max) * (1 - share)))
}

// FieldRepairable reports whether field repair would mend a slot's item: it's
// worn past what field repair restores.
func FieldRepairable(slot map[string]interface{}) bool {
	id, _ := slot["item"].(string)
	max := MaxDurability(id)
	return max > 0 && WearOf(slot) > wearAt(max, Repair().FieldUpTo)
}

// FieldRepair mends a slot's item back toward Worn: all the way on a good job
// (amount 1), half the way on a rough one (0.5). It returns the wear removed.
func FieldRepair(slot map[string]interface{}, amount float64) int {
	id, _ := slot["item"].(string)
	max := MaxDurability(id)
	target := wearAt(max, Repair().FieldUpTo)
	have := WearOf(slot)
	if max <= 0 || have <= target {
		return 0
	}
	mend := int(math.Ceil(float64(have-target) * amount))
	SetWear(slot, have-mend)
	return mend
}

// Quote is what a smith asks to mend one item.
type Quote struct {
	Gold      int    `json:"gold"`
	Material  string `json:"material,omitempty"`
	Count     int    `json:"count,omitempty"`     // of Material
	Discount  string `json:"discount,omitempty"`  // an item that cuts the price, if carried
	Percent   int    `json:"percent,omitempty"`   // ...by this much
	ToWear    int    `json:"-"`                   // the wear it'll have after
	Pristine  bool   `json:"pristine"`            // made new (or only back to Worn)
	CantWork  bool   `json:"cant_work,omitempty"` // not their material
	Untouched bool   `json:"-"`                   // nothing to mend
}

// SmithQuote is what the smith asks for a slot's item, worth value gold new.
func SmithQuote(deal SmithDeal, slot map[string]interface{}, value int) Quote {
	id, _ := slot["item"].(string)
	max := MaxDurability(id)
	have := WearOf(slot)
	q := Quote{Pristine: deal.UpTo != "worn"}
	works := false
	for _, m := range deal.Materials {
		if m == MaterialOf(id) {
			works = true
		}
	}
	if !works {
		q.CantWork = true
		return q
	}
	if !q.Pristine {
		q.ToWear = wearAt(max, Repair().FieldUpTo)
	}
	if max <= 0 || have <= q.ToWear {
		q.Untouched = true
		return q
	}
	share := float64(have-q.ToWear) / float64(max)
	q.Gold = max1(int(math.Ceil(float64(value) * share * deal.Cost)))
	if deal.Material != "" {
		q.Material, q.Count = deal.Material, int(math.Ceil(share/0.25))
	}
	if deal.Discount != nil {
		q.Discount, q.Percent = deal.Discount.Item, deal.Discount.Percent
	}
	return q
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
