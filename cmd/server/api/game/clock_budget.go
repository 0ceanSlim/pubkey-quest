package game

import (
	"time"

	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// ─── The clock's pace (beta exploit audit D4) ────────────────────────────────
//
// The client runs the clock (src/systems/smoothClock.js: 144× real time, so
// 2.4 game minutes a real second, slower in town) and reports it with each
// update_time tick. The server never took its word for how far: a client
// that said "three days later" got three days, and every daily limit with
// them. Now each tick may move the world only as far as the real time since
// the last one allows, with some slack for a slow tick, and a short bank so a
// pause can't be saved up and spent at once.

const (
	gameMinutesPerRealSecond = 2.4  // 144× real time
	clockSlack               = 1.25 // a late tick, a jittery timer
	clockBank                = 30.0 // game minutes a tick can carry over
)

// clampTick limits an update_time tick's params to the clock's budget, and
// spends it. now is passed for tests.
func clampTick(sess *session.GameSession, state *types.SaveFile, params map[string]interface{}, now time.Time) {
	ms := now.UnixMilli()
	if sess.ClockBudgetAt == 0 {
		sess.ClockBudget = clockBank
	} else if ms > sess.ClockBudgetAt {
		sess.ClockBudget += float64(ms-sess.ClockBudgetAt) / 1000 * gameMinutesPerRealSecond * clockSlack
	}
	sess.ClockBudgetAt = ms
	sess.ClockBudget = min(sess.ClockBudget, clockBank)

	day, okDay := params["current_day"].(float64)
	tod, okTime := params["time_of_day"].(float64)
	if !okDay || !okTime {
		return
	}
	from := state.CurrentDay*1440 + state.TimeOfDay
	want := int(day)*1440 + int(tod) - from
	if want <= 0 {
		return
	}
	if allow := int(sess.ClockBudget); want > allow {
		want = allow
		to := from + want
		params["current_day"], params["time_of_day"] = float64(to/1440), float64(to%1440)
	}
	sess.ClockBudget -= float64(want)
}
