package world

import (
	"testing"
	"time"
)

func TestTickItemRestock(t *testing.T) {
	start := time.Now()
	item := &MerchantInventoryItem{ItemID: "healing", CurrentStock: 0, MaxStock: 2, RestockRate: 1, RestockMinutes: 30, LastRestock: start}

	tickItemRestock(item, start.Add(29*time.Minute))
	if item.CurrentStock != 0 {
		t.Fatalf("restocked early: %d", item.CurrentStock)
	}
	tickItemRestock(item, start.Add(31*time.Minute))
	if item.CurrentStock != 1 {
		t.Fatalf("after one period want 1, got %d", item.CurrentStock)
	}
	tickItemRestock(item, start.Add(200*time.Minute))
	if item.CurrentStock != 2 {
		t.Fatalf("capped at max: want 2, got %d", item.CurrentStock)
	}

	shopWide := &MerchantInventoryItem{ItemID: "rope", CurrentStock: 0, MaxStock: 6, LastRestock: start}
	tickItemRestock(shopWide, start.Add(time.Hour))
	if shopWide.CurrentStock != 0 {
		t.Fatalf("shop-wide item ticked on its own: %d", shopWide.CurrentStock)
	}
}
