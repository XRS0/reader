package reading

import "time"

type ActivitySignals struct {
	Visible          bool
	Focused          bool
	UserActive       bool
	SinceInteraction time.Duration
}
type IntervalAccounting struct {
	ActiveSeconds int64
	IdleSeconds   int64
	CurrentActive bool
	Elapsed       time.Duration
	Credited      time.Duration
}

// AccountInterval uses server timestamps. It only credits the portion of a
// heartbeat gap for which both the previous and current observations are active.
func AccountInterval(previous, now time.Time, previousActive bool, current ActivitySignals, maxCredit, idleThreshold time.Duration) IntervalAccounting {
	currentActive := current.Visible && current.Focused && current.UserActive && current.SinceInteraction >= 0 && current.SinceInteraction <= idleThreshold
	if !now.After(previous) {
		return IntervalAccounting{CurrentActive: currentActive}
	}
	elapsed := now.Sub(previous)
	wholeSeconds := int64(elapsed / time.Second)
	if wholeSeconds <= 0 {
		return IntervalAccounting{CurrentActive: currentActive, Elapsed: elapsed}
	}
	credited := elapsed
	if credited > maxCredit {
		credited = maxCredit
	}
	creditedSeconds := int64(credited / time.Second)
	result := IntervalAccounting{CurrentActive: currentActive, Elapsed: elapsed, Credited: credited}
	if previousActive && currentActive {
		result.ActiveSeconds = creditedSeconds
		result.IdleSeconds = wholeSeconds - creditedSeconds
	} else {
		result.IdleSeconds = wholeSeconds
	}
	return result
}
