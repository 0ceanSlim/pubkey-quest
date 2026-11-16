package auth

import (
	"testing"
	"time"
)

// A burst is allowed, a flood isn't, and the bucket refills (E3).
func TestRateLimit(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	ip := "198.51.100.1"
	for i := range rateBurst {
		if !allowRequest(ip, now) {
			t.Fatalf("refused request %d of a burst", i)
		}
	}
	if allowRequest(ip, now) {
		t.Fatal("a flood got through")
	}
	if !allowRequest(ip, now.Add(time.Second)) {
		t.Fatal("the bucket didn't refill")
	}
	if !allowRequest("198.51.100.2", now) {
		t.Fatal("another address was limited")
	}
}
