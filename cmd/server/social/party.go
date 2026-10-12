package social

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ─── Parties (docs/draft/social-plan.md §9) ──────────────────────────────────
//
// A party is up to MaxParty players who travel and fight together. The leader
// moves the party; followers' clocks snap to the leader's when they join.
// Memory only: a restart disbands every party.

// MaxParty is the most members a party can have (partners count too, later).
const MaxParty = 5

// InviteTTL is how long an invite stays open.
const InviteTTL = 2 * time.Minute

// Member is one player in a party: which of their saves is playing.
type Member struct {
	Npub   string `json:"npub"`
	SaveID string `json:"save_id"`
}

// Party is a group of players. Members[0] is always the leader.
type Party struct {
	ID        int64
	Members   []Member
	CreatedAt time.Time
}

// Leader is the party's leader.
func (p *Party) Leader() Member { return p.Members[0] }

// Invite is an open invitation.
type Invite struct {
	From    Member
	To      string // npub
	Expires time.Time
}

var (
	partyMu      sync.Mutex
	partyByNpub  = map[string]*Party{}
	invitesByTo  = map[string][]Invite{}
	nextPartyID  atomic.Int64
)

// copyParty returns a snapshot safe to read outside the lock.
func copyParty(p *Party) *Party {
	if p == nil {
		return nil
	}
	c := *p
	c.Members = append([]Member(nil), p.Members...)
	return &c
}

// PartyOf returns a snapshot of the player's party, or nil.
func PartyOf(npub string) *Party {
	partyMu.Lock()
	defer partyMu.Unlock()
	return copyParty(partyByNpub[npub])
}

// IsLeader reports whether npub leads a party.
func IsLeader(npub string) bool {
	p := PartyOf(npub)
	return p != nil && p.Leader().Npub == npub
}

// IsFollower reports whether npub is in a party they don't lead.
func IsFollower(npub string) bool {
	p := PartyOf(npub)
	return p != nil && p.Leader().Npub != npub
}

// AddInvite records an invite from one player to another. The checks that
// need presence (same spot, both online) are the caller's.
func AddInvite(from Member, to string, now time.Time) error {
	partyMu.Lock()
	defer partyMu.Unlock()
	if from.Npub == to {
		return fmt.Errorf("you can't invite yourself")
	}
	if partyByNpub[to] != nil {
		return fmt.Errorf("they're already in a party")
	}
	if p := partyByNpub[from.Npub]; p != nil {
		if p.Leader().Npub != from.Npub {
			return fmt.Errorf("only your party leader can invite")
		}
		if len(p.Members) >= MaxParty {
			return fmt.Errorf("your party is full (%d)", MaxParty)
		}
	}
	list := pruneInvites(invitesByTo[to], now)
	for i, inv := range list {
		if inv.From.Npub == from.Npub {
			list[i].Expires = now.Add(InviteTTL)
			invitesByTo[to] = list
			return nil
		}
	}
	invitesByTo[to] = append(list, Invite{From: from, To: to, Expires: now.Add(InviteTTL)})
	return nil
}

func pruneInvites(list []Invite, now time.Time) []Invite {
	out := list[:0]
	for _, inv := range list {
		if now.Before(inv.Expires) {
			out = append(out, inv)
		}
	}
	return out
}

// InvitesFor returns a player's open invites.
func InvitesFor(npub string, now time.Time) []Invite {
	partyMu.Lock()
	defer partyMu.Unlock()
	list := pruneInvites(invitesByTo[npub], now)
	invitesByTo[npub] = list
	return append([]Invite(nil), list...)
}

// TakeInvite removes and returns the invite from `from` to `to`.
func TakeInvite(to, from string, now time.Time) (Invite, bool) {
	partyMu.Lock()
	defer partyMu.Unlock()
	list := pruneInvites(invitesByTo[to], now)
	for i, inv := range list {
		if inv.From.Npub == from {
			invitesByTo[to] = append(list[:i], list[i+1:]...)
			return inv, true
		}
	}
	invitesByTo[to] = list
	return Invite{}, false
}

// Join puts joiner into the inviter's party, creating it if the inviter is
// solo. Returns the party snapshot.
func Join(inviter, joiner Member) (*Party, error) {
	partyMu.Lock()
	defer partyMu.Unlock()
	if partyByNpub[joiner.Npub] != nil {
		return nil, fmt.Errorf("you're already in a party")
	}
	p := partyByNpub[inviter.Npub]
	if p == nil {
		p = &Party{ID: nextPartyID.Add(1), Members: []Member{inviter}, CreatedAt: time.Now()}
		partyByNpub[inviter.Npub] = p
	}
	if p.Leader().Npub != inviter.Npub {
		return nil, fmt.Errorf("that invite is no longer good")
	}
	if len(p.Members) >= MaxParty {
		return nil, fmt.Errorf("the party is full")
	}
	p.Members = append(p.Members, joiner)
	partyByNpub[joiner.Npub] = p
	delete(invitesByTo, joiner.Npub)
	return copyParty(p), nil
}

// Leave takes a player out of their party. If the leader leaves, the next
// member leads; a party of one disbands. Returns the party as it is now (nil
// when it disbanded) and whether the player was in one.
func Leave(npub string) (*Party, bool) {
	partyMu.Lock()
	defer partyMu.Unlock()
	p := partyByNpub[npub]
	if p == nil {
		return nil, false
	}
	delete(partyByNpub, npub)
	for i, m := range p.Members {
		if m.Npub == npub {
			p.Members = append(p.Members[:i], p.Members[i+1:]...)
			break
		}
	}
	if len(p.Members) <= 1 {
		for _, m := range p.Members {
			delete(partyByNpub, m.Npub)
		}
		return nil, true
	}
	return copyParty(p), true
}

// Promote makes another member the leader (leader only).
func Promote(leader, to string) (*Party, error) {
	partyMu.Lock()
	defer partyMu.Unlock()
	p := partyByNpub[leader]
	if p == nil || p.Leader().Npub != leader {
		return nil, fmt.Errorf("only the party leader can do that")
	}
	for i, m := range p.Members {
		if m.Npub == to && i > 0 {
			p.Members[0], p.Members[i] = p.Members[i], p.Members[0]
			return copyParty(p), nil
		}
	}
	return nil, fmt.Errorf("they're not in your party")
}

// Kick removes a member (leader only).
func Kick(leader, who string) (*Party, error) {
	partyMu.Lock()
	p := partyByNpub[leader]
	if p == nil || p.Leader().Npub != leader {
		partyMu.Unlock()
		return nil, fmt.Errorf("only the party leader can do that")
	}
	if who == leader {
		partyMu.Unlock()
		return nil, fmt.Errorf("leave the party instead")
	}
	found := false
	for _, m := range p.Members {
		if m.Npub == who {
			found = true
		}
	}
	partyMu.Unlock()
	if !found {
		return nil, fmt.Errorf("they're not in your party")
	}
	np, _ := Leave(who)
	return np, nil
}

// ResetForTests clears all parties and invites.
func ResetForTests() {
	partyMu.Lock()
	partyByNpub = map[string]*Party{}
	invitesByTo = map[string][]Invite{}
	partyMu.Unlock()
}
