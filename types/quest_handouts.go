package types

// HandedBack reports whether an item a stage gives (stage index i) is taken
// back by a later stage of the same quest (a fetch objective that consumes
// it): Thistle's incense, burned in the ritual. Such items are hand-ins, not
// rewards, and shouldn't be listed as what the quest pays.
func (q *QuestData) HandedBack(i int, itemID string) bool {
	for _, st := range q.Stages[i+1:] {
		for _, o := range st.Objectives {
			if o.Consume && o.Target == itemID {
				return true
			}
		}
	}
	return false
}
