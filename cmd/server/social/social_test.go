package social

import (
	"strings"
	"testing"
	"time"
)

func TestPartyLifecycle(t *testing.T) {
	ResetForTests()
	now := time.Now()
	a, b, c := Member{"a", "s1"}, Member{"b", "s2"}, Member{"c", "s3"}

	if err := AddInvite(a, "b", now); err != nil {
		t.Fatal(err)
	}
	if len(InvitesFor("b", now)) != 1 {
		t.Fatal("invite not listed")
	}
	if _, ok := TakeInvite("b", "a", now); !ok {
		t.Fatal("couldn't take invite")
	}
	p, err := Join(a, b)
	if err != nil || len(p.Members) != 2 || p.Leader().Npub != "a" {
		t.Fatalf("join: %v %+v", err, p)
	}
	if !IsLeader("a") || !IsFollower("b") {
		t.Fatal("roles wrong")
	}
	// Followers can't invite; the leader can.
	if err := AddInvite(b, "c", now); err == nil {
		t.Error("a follower invited someone")
	}
	if _, err := Join(a, c); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote("a", "c"); err != nil || !IsLeader("c") {
		t.Fatalf("promote: %v", err)
	}
	// Leader leaves → next member leads.
	p, _ = Leave("c")
	if p == nil || p.Leader().Npub != "b" { // promote swapped a to the back: [c b a] → [b a]
		t.Fatalf("after leader left: %+v", p)
	}
	// Down to one → disbanded.
	if p, _ = Leave("b"); p != nil || PartyOf("a") != nil {
		t.Errorf("party of one should disband, got %+v", p)
	}
}

func TestInviteExpiresAndPartyCap(t *testing.T) {
	ResetForTests()
	now := time.Now()
	if err := AddInvite(Member{"a", "1"}, "b", now); err != nil {
		t.Fatal(err)
	}
	if got := InvitesFor("b", now.Add(InviteTTL+time.Second)); len(got) != 0 {
		t.Error("invite outlived its TTL")
	}
	lead := Member{"lead", "x"}
	for i := 0; i < MaxParty-1; i++ {
		if _, err := Join(lead, Member{string(rune('m' + i)), "x"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := AddInvite(lead, "late", now); err == nil {
		t.Error("invited into a full party")
	}
}

func TestChatCleanAndRateLimit(t *testing.T) {
	if CleanText("  hi\x07 there \n") != "hi there" {
		t.Errorf("CleanText = %q", CleanText("  hi\x07 there \n"))
	}
	if n := len([]rune(CleanText(strings.Repeat("é", 500)))); n != MaxChatLength {
		t.Errorf("length cap: %d", n)
	}
	now := time.Now()
	for i := 0; i < 5; i++ {
		if !AllowSend("spammer", now) {
			t.Fatalf("line %d refused", i)
		}
	}
	if AllowSend("spammer", now) {
		t.Error("sixth line in 10 s allowed")
	}
	if !AllowSend("spammer", now.Add(11*time.Second)) {
		t.Error("still limited after the window")
	}
	Deliver(ChatLine{Channel: "say", Text: "x"}, []string{"r1", "r2"})
	if len(Drain("r1")) != 1 || len(Drain("r1")) != 0 {
		t.Error("drain should return the line once")
	}
}
