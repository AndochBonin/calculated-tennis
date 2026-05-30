package tennisabstract

import (
	"fmt"
	"time"

	"github.com/AndochBonin/E3/tennis/models"
)

const backtestSeasonMatchCount = 100

// TourneyDateAsTime converts a Sackmann YYYYMMDD tourney_date to UTC midnight.
func TourneyDateAsTime(yyyymmdd int) (time.Time, bool) {
	if yyyymmdd < 10101 {
		return time.Time{}, false
	}
	if _, ok := parseTourneyDate(fmt.Sprintf("%08d", yyyymmdd)); !ok {
		return time.Time{}, false
	}
	y := yyyymmdd / 10000
	m := (yyyymmdd / 100) % 100
	d := yyyymmdd % 100
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC), true
}

// PlayerStatsForBacktestForm builds minimal PlayerStats for form adjustment when only
// precomputed season hold/break rates are available (e.g. player_rates_2024.json).
func PlayerStatsForBacktestForm(slug string, asOf time.Time, hold, breakPct, dr float64) models.PlayerStats {
	year := asOf.Year()
	return models.PlayerStats{
		PlayerSlug: slug,
		TourLevelSeasons: []models.TourLevelSeason{{
			Year:     year,
			Matches:  backtestSeasonMatchCount,
			HoldPct:  hold,
			BreakPct: breakPct,
			DR:       dr,
		}},
	}
}

// AdjustedHoldBreakAsOf applies recent-form adjustment using career results filtered
// strictly before asOf, then delegates to AdjustedHoldBreak for season baseline + blend.
func AdjustedHoldBreakAsOf(stats models.PlayerStats, careerRecent []models.RecentResult, opts FormOptions) (AdjustedRates, error) {
	opts = opts.withDefaults()
	asOf := opts.asOf(stats)
	stats.RecentResults = RecentResultsBefore(careerRecent, asOf, opts.RecentMatchLimit)
	opts.AsOf = asOf
	return AdjustedHoldBreak(stats, opts)
}
