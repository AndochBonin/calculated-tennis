package tennisabstract

import (
	"math"
	"sort"
	"time"

	"github.com/AndochBonin/polymarket/models"
)

// FormOptions tunes hold/break form adjustment; zero values use documented defaults.
type FormOptions struct {
	AsOf             time.Time // evaluation date; zero → FetchedAt or time.Now()
	MinSeasonMatches int       // default 20: blend current + prior season when below
	RecentMatchLimit int       // default 15: max recent rows with DR considered
	HalfLifeMatches  float64   // default 5: exp decay half-life (index 0 = most recent)
	FormWeightMax    float64   // default 0.30: cap γ (share from form-adjusted term)
	FormRatioMin     float64   // default 0.92: clamp DR_form/DR_season
	FormRatioMax     float64   // default 1.08
}

// AdjustedRates holds form-adjusted hold/break and optional diagnostics.
type AdjustedRates struct {
	HoldPct, BreakPct float64
	SeasonHold        float64
	SeasonBreak       float64
	DRSeason          float64
	DRForm            float64
	FormRatio         float64
	FormWeight        float64
	SeasonMatches     int
}

// AdjustedHoldBreak computes form-aware hold and break rates from scraped stats.
// Season baseline uses the calendar year of AsOf (or FetchedAt / now); recent form
// is a DR-weighted EWMA that scales hold/break via a convex blend.
func AdjustedHoldBreak(stats models.PlayerStats, opts FormOptions) (AdjustedRates, error) {
	opts = opts.withDefaults()
	asOf := opts.asOf(stats)

	year := asOf.Year()
	curr, ok := findSeason(stats.TourLevelSeasons, year)
	if !ok {
		return AdjustedRates{}, ErrNoSeasonData
	}

	h0, b0, dr0 := curr.HoldPct, curr.BreakPct, curr.DR
	matches := curr.Matches

	if matches < opts.MinSeasonMatches {
		if prev, ok := findSeason(stats.TourLevelSeasons, year-1); ok {
			w := float64(matches) / float64(matches+prev.Matches)
			h0 = w*curr.HoldPct + (1-w)*prev.HoldPct
			b0 = w*curr.BreakPct + (1-w)*prev.BreakPct
			dr0 = w*curr.DR + (1-w)*prev.DR
			matches = matches + prev.Matches
		}
	}

	drForm, nValid := recentDRForm(stats.RecentResults, opts.RecentMatchLimit, opts.HalfLifeMatches)

	r := 1.0
	if dr0 != 0 && nValid > 0 {
		r = drForm / dr0
		if r < opts.FormRatioMin {
			r = opts.FormRatioMin
		}
		if r > opts.FormRatioMax {
			r = opts.FormRatioMax
		}
	}

	gamma := 0.0
	if nValid > 0 {
		gamma = opts.FormWeightMax
		if lim := float64(opts.RecentMatchLimit); lim > 0 {
			if float64(nValid)/lim < 1 {
				gamma *= float64(nValid) / lim
			}
		}
	}

	hAdj := (1-gamma)*h0 + gamma*(h0*r)
	bAdj := (1-gamma)*b0 + gamma*(b0*r)

	return AdjustedRates{
		HoldPct:       hAdj,
		BreakPct:      bAdj,
		SeasonHold:    h0,
		SeasonBreak:   b0,
		DRSeason:      dr0,
		DRForm:        drForm,
		FormRatio:     r,
		FormWeight:    gamma,
		SeasonMatches: matches,
	}, nil
}

func (o FormOptions) withDefaults() FormOptions {
	if o.MinSeasonMatches <= 0 {
		o.MinSeasonMatches = 20
	}
	if o.RecentMatchLimit <= 0 {
		o.RecentMatchLimit = 15
	}
	if o.HalfLifeMatches <= 0 {
		o.HalfLifeMatches = 5
	}
	if o.FormWeightMax <= 0 {
		o.FormWeightMax = 0.30
	}
	if o.FormRatioMin <= 0 {
		o.FormRatioMin = 0.92
	}
	if o.FormRatioMax <= 0 {
		o.FormRatioMax = 1.08
	}
	return o
}

func (o FormOptions) asOf(stats models.PlayerStats) time.Time {
	if !o.AsOf.IsZero() {
		return o.AsOf
	}
	if !stats.FetchedAt.IsZero() {
		return stats.FetchedAt
	}
	return time.Now()
}

func findSeason(seasons []models.TourLevelSeason, year int) (models.TourLevelSeason, bool) {
	for _, s := range seasons {
		if s.IsCareer || s.Year != year {
			continue
		}
		return s, true
	}
	return models.TourLevelSeason{}, false
}

func recentDRForm(recent []models.RecentResult, limit int, halfLife float64) (mean float64, nValid int) {
	if limit <= 0 || len(recent) == 0 {
		return 0, 0
	}

	sorted := append([]models.RecentResult(nil), recent...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.After(sorted[j].Date)
	})

	var sumW, sumDR float64
	for _, row := range sorted {
		if row.DominanceRatio == nil {
			continue
		}
		w := math.Pow(0.5, float64(nValid)/halfLife)
		sumW += w
		sumDR += w * *row.DominanceRatio
		nValid++
		if nValid >= limit {
			break
		}
	}
	if nValid == 0 || sumW == 0 {
		return 0, 0
	}
	return sumDR / sumW, nValid
}
