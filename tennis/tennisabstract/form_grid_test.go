package tennisabstract

import (
	"math"
	"testing"
)

func TestFloatGrid_ratioMinRange(t *testing.T) {
	got := FloatGrid(0.80, 1.00, 0.04)
	want := []float64{0.80, 0.84, 0.88, 0.92, 0.96, 1.00}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("[%d]=%v, want %v (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestFloatGrid_inclusiveStop(t *testing.T) {
	got := FloatGrid(0, 1, 0.10)
	if len(got) != 11 {
		t.Fatalf("len=%d, want 11: %v", len(got), got)
	}
	if got[0] != 0 || math.Abs(got[len(got)-1]-1) > 1e-12 {
		t.Fatalf("endpoints: first=%v last=%v", got[0], got[len(got)-1])
	}
}

func TestFloatGrid_invalid(t *testing.T) {
	if FloatGrid(1, 0, 0.1) != nil {
		t.Fatal("start > stop should return nil")
	}
	if FloatGrid(0, 1, 0) != nil {
		t.Fatal("step <= 0 should return nil")
	}
}

func TestFormGridCombos_defaultBounds(t *testing.T) {
	bounds := DefaultFormGridBounds()
	combos := FormGridCombos(bounds)

	halfLives := FloatGrid(bounds.HalfLifeStart, bounds.HalfLifeStop, bounds.HalfLifeStep)
	weightMaxes := FloatGrid(bounds.WeightMaxStart, bounds.WeightMaxStop, bounds.WeightMaxStep)
	ratioMins := FloatGrid(bounds.RatioMinStart, bounds.RatioMinStop, bounds.RatioMinStep)
	ratioMaxes := FloatGrid(bounds.RatioMaxStart, bounds.RatioMaxStop, bounds.RatioMaxStep)

	if len(halfLives) != 5 {
		t.Fatalf("half-life grid len=%d, want 5", len(halfLives))
	}
	if len(weightMaxes) != 11 {
		t.Fatalf("weight-max grid len=%d, want 11", len(weightMaxes))
	}
	if len(ratioMins) != 6 {
		t.Fatalf("ratio-min grid len=%d, want 6", len(ratioMins))
	}
	if len(ratioMaxes) != 6 {
		t.Fatalf("ratio-max grid len=%d, want 6", len(ratioMaxes))
	}

	validRatioPairs := 0
	for _, rmin := range ratioMins {
		for _, rmax := range ratioMaxes {
			if rmin < rmax {
				validRatioPairs++
			}
		}
	}
	if validRatioPairs != 35 {
		t.Fatalf("valid ratio pairs=%d, want 35", validRatioPairs)
	}

	wantCombos := len(halfLives) * len(weightMaxes) * validRatioPairs
	if len(combos) != wantCombos {
		t.Fatalf("combos len=%d, want %d", len(combos), wantCombos)
	}
	if len(combos) != 1925 {
		t.Fatalf("combos len=%d, want 1925", len(combos))
	}

	for i, o := range combos {
		if o.FormRatioMin >= o.FormRatioMax {
			t.Fatalf("combo %d: ratio_min=%v >= ratio_max=%v", i, o.FormRatioMin, o.FormRatioMax)
		}
		if o.HalfLifeMatches <= 0 || o.FormRatioMin <= 0 || o.FormRatioMax <= 0 {
			t.Fatalf("combo %d: unexpected non-positive tuned field %+v", i, o)
		}
	}
}

func TestFormGridCombos_explicitZeroWeight(t *testing.T) {
	bounds := FormGridBounds{
		HalfLifeStart: 5, HalfLifeStop: 5, HalfLifeStep: 1,
		WeightMaxStart: 0, WeightMaxStop: 0, WeightMaxStep: 0.10,
		RatioMinStart: 0.92, RatioMinStop: 0.92, RatioMinStep: 0.04,
		RatioMaxStart: 1.08, RatioMaxStop: 1.08, RatioMaxStep: 0.04,
	}
	combos := FormGridCombos(bounds)
	if len(combos) != 1 {
		t.Fatalf("len=%d, want 1", len(combos))
	}
	if combos[0].FormWeightMax != 0 {
		t.Fatalf("FormWeightMax=%v, want 0", combos[0].FormWeightMax)
	}
}
