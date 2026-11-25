package usage

import (
	"strings"
	"time"

	"github.com/zahT3/hotseat/internal/account"
)

// A limit with no known reset time blocks an account for one 5-hour window.
const unknownLimitHold = 5 * time.Hour

// Recommend picks the account to take over from exclude: the one with the
// most quota left, skipping accounts that hit a limit that has not reset.
func Recommend(reg account.Registry, records map[string]Record, exclude string, now time.Time) (string, bool) {
	return RecommendFor(reg, records, exclude, "", now)
}

// RecommendFor is Recommend for a session that hit a limit on model: an
// account that is itself limited on that model is skipped too. An empty
// model is plain Recommend.
func RecommendFor(reg account.Registry, records map[string]Record, exclude, model string, now time.Time) (string, bool) {
	best, bestScore := "", 0.0
	for _, a := range reg.Accounts {
		if a.Name == exclude {
			continue
		}
		r := records[a.Name]
		if r.LimitActive(now) || (model != "" && r.ModelLimited(model, now)) {
			continue
		}
		five, _, _ := Effective(r.FiveHour, now)
		seven, _, _ := Effective(r.SevenDay, now)
		score := max(five, seven)
		if best == "" || score < bestScore {
			best, bestScore = a.Name, score
		}
	}
	return best, best != ""
}

// LimitActive reports whether the account is still blocked: its limit has
// not reset, and nothing seen since says otherwise. Usage windows observed
// after the limit, all below 100%, mean it is working again (the reset
// time in a limit message can be wrong or belong to another window).
func (r Record) LimitActive(now time.Time) bool {
	if !r.Limit.Active(now) {
		return false
	}
	seenAfter := false
	for _, w := range []*Window{r.FiveHour, r.SevenDay} {
		if w == nil || w.ObservedAt <= r.Limit.ObservedAt {
			continue
		}
		if w.UsedPercent >= 100 {
			return true
		}
		seenAfter = true
	}
	return !seenAfter
}

// ModelLimited reports whether the account's limit on model still holds.
func (r Record) ModelLimited(model string, now time.Time) bool {
	return r.ModelLimit != nil && strings.EqualFold(r.ModelLimit.Model, model) && r.ModelLimit.Active(now)
}

// Active reports whether the account is still blocked by this limit.
func (l *LimitHit) Active(now time.Time) bool {
	if l == nil {
		return false
	}
	if l.ResetsAt > 0 {
		return l.ResetsAt > now.Unix()
	}
	return now.Sub(time.Unix(l.ObservedAt, 0)) < unknownLimitHold
}
