package tennisabstract

import "math"

const floatGridStopEpsilon = 1e-9

// FormGridBounds defines inclusive ranges for form-parameter grid search.
type FormGridBounds struct {
	HalfLifeStart, HalfLifeStop, HalfLifeStep float64
	WeightMaxStart, WeightMaxStop, WeightMaxStep float64
	RatioMinStart, RatioMinStop, RatioMinStep float64
	RatioMaxStart, RatioMaxStop, RatioMaxStep float64
}

// DefaultFormGridBounds returns the standard calibrate-form grid ranges.
func DefaultFormGridBounds() FormGridBounds {
	return FormGridBounds{
		HalfLifeStart: 1, HalfLifeStop: 10, HalfLifeStep: 2,
		WeightMaxStart: 0, WeightMaxStop: 1, WeightMaxStep: 0.10,
		RatioMinStart: 0.80, RatioMinStop: 1.00, RatioMinStep: 0.04,
		RatioMaxStart: 1.00, RatioMaxStop: 1.20, RatioMaxStep: 0.04,
	}
}

// FloatGrid returns start, start+step, …, stop (inclusive). Returns nil when step <= 0
// or start > stop.
func FloatGrid(start, stop, step float64) []float64 {
	if step <= 0 || start > stop {
		return nil
	}
	n := int(math.Floor((stop-start)/step+floatGridStopEpsilon)) + 1
	if n <= 0 {
		return nil
	}
	out := make([]float64, n)
	for i := range out {
		out[i] = start + float64(i)*step
	}
	last := out[n-1]
	if math.Abs(last-stop) <= floatGridStopEpsilon {
		out[n-1] = stop
	}
	return out
}

// FormGridCombos enumerates the Cartesian product of the four tuned form dimensions,
// keeping only points where FormRatioMin < FormRatioMax. Each combo sets
// HalfLifeMatches, FormWeightMax, FormRatioMin, and FormRatioMax explicitly (including
// FormWeightMax=0 for no form blend).
func FormGridCombos(bounds FormGridBounds) []FormOptions {
	halfLives := FloatGrid(bounds.HalfLifeStart, bounds.HalfLifeStop, bounds.HalfLifeStep)
	weightMaxes := FloatGrid(bounds.WeightMaxStart, bounds.WeightMaxStop, bounds.WeightMaxStep)
	ratioMins := FloatGrid(bounds.RatioMinStart, bounds.RatioMinStop, bounds.RatioMinStep)
	ratioMaxes := FloatGrid(bounds.RatioMaxStart, bounds.RatioMaxStop, bounds.RatioMaxStep)

	n := len(halfLives) * len(weightMaxes) * len(ratioMins) * len(ratioMaxes)
	combos := make([]FormOptions, 0, n/2)
	for _, hl := range halfLives {
		for _, w := range weightMaxes {
			for _, rmin := range ratioMins {
				for _, rmax := range ratioMaxes {
					if rmin >= rmax {
						continue
					}
					combos = append(combos, FormOptions{
						HalfLifeMatches: hl,
						FormWeightMax:   w,
						FormRatioMin:    rmin,
						FormRatioMax:    rmax,
					})
				}
			}
		}
	}
	return combos
}
