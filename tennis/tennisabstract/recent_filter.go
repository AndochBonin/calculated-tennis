package tennisabstract

import (
	"sort"
	"time"

	"github.com/AndochBonin/E3/tennis/models"
)

// RecentResultsBefore returns up to limit matches whose calendar date is strictly
// before asOf (UTC date-only). Results are sorted newest-first; equal dates keep
// the relative order from the input slice.
func RecentResultsBefore(results []models.RecentResult, asOf time.Time, limit int) []models.RecentResult {
	if limit <= 0 || len(results) == 0 {
		return nil
	}

	cutoff := utcDateOnly(asOf)
	filtered := make([]models.RecentResult, 0, len(results))
	for _, r := range results {
		if !utcDateOnly(r.Date).Before(cutoff) {
			continue
		}
		filtered = append(filtered, r)
	}
	if len(filtered) == 0 {
		return nil
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Date.After(filtered[j].Date)
	})
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func utcDateOnly(t time.Time) time.Time {
	t = t.In(time.UTC)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
