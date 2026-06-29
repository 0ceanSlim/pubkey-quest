package poi

import (
	"slices"

	"pubkey-quest/types"
)

// Session is the in-memory state of a node walk in progress — a discovered POI
// or a fired encounter (both share the POIStep node schema, so one session type
// drives both). The server holds one on the game session (next to the combat
// session); it is never written to the save file. The walker itself (Resolve) is
// stateless — this struct lets the server resolve a node, hand the result to the
// player, and resolve the next one when they choose. The node graph is carried
// here (Nodes) so the walk is self-contained and source-agnostic.
type Session struct {
	POIID       string                  // id of the POI/encounter being walked (for logging)
	Title       string                  // display name (POI/encounter name)
	Nodes       map[string]types.POIStep // the graph being walked (from a POI or an encounter)
	CurrentNode string                  // the node last resolved and shown to the player
	ValidNexts  []string                // anti-skip allowlist: the only node ids /advance accepts next
	ResumeNext  string                  // set when a monster node bridged into combat; resolved on victory
}

// AllowsNext reports whether next is a node the player may legitimately advance
// to from the current node — the anti-skip guard. An empty ValidNexts (terminal
// node, or a monster node whose combat is driving the walk) allows nothing.
func (s *Session) AllowsNext(next string) bool {
	return slices.Contains(s.ValidNexts, next)
}

// NextsFor derives the anti-skip allowlist from a resolved step: every offered
// choice on a choice node, or the single Continue target otherwise. Terminal
// nodes and monster nodes (combat drives the walk) yield nothing.
func NextsFor(res StepResult) []string {
	if res.Terminal || res.Combat != "" {
		return nil
	}
	if len(res.Choices) > 0 {
		nexts := make([]string, 0, len(res.Choices))
		for _, ch := range res.Choices {
			nexts = append(nexts, ch.Next)
		}
		return nexts
	}
	if res.Next != "" {
		return []string{res.Next}
	}
	return nil
}
