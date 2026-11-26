package tennisabstract

import (
	"math"
	"sort"
	"time"

	"github.com/AndochBonin/calculated-tennis/tennis/models"
)

const (
	defaultMinSeasonMatches = 20
	defaultRecentMatchLimit = 15
	defaultHalfLifeMatches  = 5.0
	defaultFormWeightMax    = 0.30
	defaultFormRatioMin     = 0.92
	defaultFormRatioMax     = 1.08
	defaultChallengerWeight = 0.7
)

// FormOptions tunes hold/break form adjustment; zero values use documented defaults.
// FormWeightMax: unset (0 with no other tuning fields) → default 0.30; explicit 0
// (with HalfLifeMatches, FormRatioMin, and FormRatioMax set) → γ = 0.
type FormOptions struct {
	AsOf             time.Time // evaluation date; zero → FetchedAt or time.Now()
	MinSeasonMatches int       // default 20: blend current + prior season when below
	RecentMatchLimit int       // default 15: max recent rows with DR considered
	HalfLifeMatches  float64   // default 5: exp decay half-life (index 0 = most recent)
	FormWeightMax    float64   // cap γ; see package comment on explicit zero
	FormRatioMin     float64   // default 0.92: clamp DR_form/DR_season
	FormRatioMax     float64   // default 1.08
	// ChallengerWeight multiplies challenger HoldPct, BreakPct, and DR before they enter the
	// evaluation-year baseline (challenger-only or match-weighted mix with a thin tour year).
	// Match counts are unchanged. Default 0.7.
	ChallengerWeight float64
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
	curr, matches, ok := buildEffectiveCurrentSeason(
		stats.TourLevelSeasons, stats.ChallengerSeasons, year, opts.MinSeasonMatches, opts.ChallengerWeight,
	)
	if !ok {
		return AdjustedRates{}, ErrNoSeasonData
	}

	h0, b0, dr0 := curr.HoldPct, curr.BreakPct, curr.DR

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
	if nValid > 0 {
		denom := dr0
		if denom == 0 {
			denom = 1.0
		}
		r = drForm / denom
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

	hAdj := clampUnitInterval((1-gamma)*h0 + gamma*(h0*r))
	bAdj := clampUnitInterval((1-gamma)*b0 + gamma*(b0*r))

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
		// Blend current + prior season when the evaluation year has fewer matches.
		o.MinSeasonMatches = defaultMinSeasonMatches
	}
	if o.RecentMatchLimit <= 0 {
		// Cap how many recent rows with dominance ratio feed the EWMA.
		o.RecentMatchLimit = defaultRecentMatchLimit
	}
	if o.HalfLifeMatches <= 0 {
		// Exponential decay half-life in match index (0 = most recent).
		o.HalfLifeMatches = defaultHalfLifeMatches
	}
	if o.FormWeightMax < 0 || (o.FormWeightMax == 0 && !formWeightMaxExplicitZero(o)) {
		// Upper bound on γ, the share from form-adjusted hold/break.
		o.FormWeightMax = defaultFormWeightMax
	}
	if o.FormRatioMin <= 0 {
		// Floor on DR_form / DR_season when scaling hold and break.
		o.FormRatioMin = defaultFormRatioMin
	}
	if o.FormRatioMax <= 0 {
		// Ceiling on DR_form / DR_season when scaling hold and break.
		o.FormRatioMax = defaultFormRatioMax
	}
	if o.ChallengerWeight <= 0 {
		o.ChallengerWeight = defaultChallengerWeight
	}
	return o
}

// formWeightMaxExplicitZero is true when callers set all tuned form fields including FormWeightMax=0 to disable the form blend.
func formWeightMaxExplicitZero(o FormOptions) bool {
	return o.FormWeightMax == 0 &&
		o.HalfLifeMatches > 0 && o.FormRatioMin > 0 && o.FormRatioMax > 0
}

// buildEffectiveCurrentSeason resolves the evaluation-year baseline from tour and,
// when allowed, challenger rows.
//
// Tour with at least minMatches matches stands alone. Otherwise challenger may supplement
// only when the latest non-career challenger year equals year. chalWeight scales challenger
// hold, break, and DR (via scaleChallengerSeason); match counts are never scaled.
//
//   - Challenger-only: scaled rates, raw challenger match count.
//   - Thin tour + challenger: w = tourM/(tourM+chalM), rates = w*tour + (1-w)*scaledChal.
func buildEffectiveCurrentSeason(
	tour, challenger []models.TourLevelSeason,
	year, minMatches int,
	chalWeight float64,
) (season models.TourLevelSeason, matches int, ok bool) {
	tourRow, hasTour := findSeason(tour, year)
	chalRow, hasChalYear := findSeason(challenger, year)
	useChal := mostRecentChallengerYear(challenger) == year && hasChalYear

	if hasTour && tourRow.Matches >= minMatches {
		return tourRow, tourRow.Matches, true
	}

	if useChal {
		if !hasTour {
			scaled := scaleChallengerSeason(chalRow, chalWeight)
			return scaled, scaled.Matches, true
		}
		chal := scaleChallengerSeason(chalRow, chalWeight)
		w := float64(tourRow.Matches) / float64(tourRow.Matches+chalRow.Matches)
		blended := tourRow
		blended.HoldPct = w*tourRow.HoldPct + (1-w)*chal.HoldPct
		blended.BreakPct = w*tourRow.BreakPct + (1-w)*chal.BreakPct
		blended.DR = w*tourRow.DR + (1-w)*chal.DR
		matches := tourRow.Matches + chalRow.Matches
		blended.Matches = matches
		return blended, matches, true
	}

	if hasTour {
		return tourRow, tourRow.Matches, true
	}
	return models.TourLevelSeason{}, 0, false
}

// scaleChallengerSeason applies chalWeight to hold, break, and DR; Matches is unchanged.
func scaleChallengerSeason(s models.TourLevelSeason, weight float64) models.TourLevelSeason {
	s.HoldPct *= weight
	s.BreakPct *= weight
	s.DR *= weight
	return s
}

func mostRecentChallengerYear(seasons []models.TourLevelSeason) int {
	max := 0
	for _, s := range seasons {
		if s.IsCareer {
			continue
		}
		if s.Year > max {
			max = s.Year
		}
	}
	return max
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

func clampUnitInterval(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
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
