package tennisabstract

import (
	"context"
	"time"

	"github.com/AndochBonin/calculated-tennis/tennis/models"
)

// PlayerRecentKey identifies recent results for a player strictly before a tourney date.
type PlayerRecentKey struct {
	Slug        string
	TourneyDate int // YYYYMMDD
}

// CalibrationMatchRecent holds pre-fetched recent rows for both players in one match.
type CalibrationMatchRecent struct {
	RecentA []models.RecentResult
	RecentB []models.RecentResult
	OK      bool
}

// CalibrationRecentPreload stores per-match recent results for a form calibration surface.
// Build once with PreloadCalibrationMatchRecent before iterating the form grid.
type CalibrationRecentPreload struct {
	PerMatch []CalibrationMatchRecent
}

// PreloadCalibrationMatchRecent fetches career recent results for every player in matches,
// deduplicating by (slug, tourney date). recentLimit <= 0 uses FormOptions defaults.
func PreloadCalibrationMatchRecent(
	ctx context.Context,
	client *Client,
	matches []CalibrationMatch,
	recentLimit int,
) CalibrationRecentPreload {
	out := CalibrationRecentPreload{PerMatch: make([]CalibrationMatchRecent, len(matches))}
	if client == nil || len(matches) == 0 {
		return out
	}
	if recentLimit <= 0 {
		recentLimit = FormOptions{}.withDefaults().RecentMatchLimit
	}

	cache := make(map[PlayerRecentKey][]models.RecentResult)
	for i, m := range matches {
		asOf, ok := TourneyDateAsTime(m.TourneyDate)
		if !ok {
			continue
		}
		ra, okA := preloadPlayerRecent(ctx, client, cache, m.PlayerA, m.PlayerASlug, m.TourneyDate, asOf, recentLimit)
		rb, okB := preloadPlayerRecent(ctx, client, cache, m.PlayerB, m.PlayerBSlug, m.TourneyDate, asOf, recentLimit)
		out.PerMatch[i] = CalibrationMatchRecent{RecentA: ra, RecentB: rb, OK: okA && okB}
	}
	return out
}

func preloadPlayerRecent(
	ctx context.Context,
	client *Client,
	cache map[PlayerRecentKey][]models.RecentResult,
	playerName, slug string,
	tourneyDate int,
	asOf time.Time,
	limit int,
) ([]models.RecentResult, bool) {
	key := PlayerRecentKey{Slug: slug, TourneyDate: tourneyDate}
	if recent, ok := cache[key]; ok {
		return recent, true
	}
	recent, err := client.GetRecentResultsAsOf(ctx, playerName, asOf, limit)
	if err != nil {
		return nil, false
	}
	cache[key] = recent
	return recent, true
}
