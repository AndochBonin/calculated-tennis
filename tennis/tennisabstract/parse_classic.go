package tennisabstract

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/AndochBonin/E3/tennis/models"
)

// matchmx column indices (var matchhead on player-classic.cgi).
const (
	matchColDate      = 0
	matchColTourn     = 1
	matchColSurf      = 2
	matchColLevel     = 3
	matchColWL        = 4
	matchColRank      = 5
	matchColRound     = 8
	matchColScore     = 9
	matchColOpp       = 11
	matchColORank     = 12
	matchColTime      = 20
	matchColAces      = 21
	matchColDFs       = 22
	matchColPts       = 23
	matchColFirsts    = 24
	matchColFWon      = 25
	matchColSWon      = 26
	matchColSaved     = 28
	matchColChances   = 29
	matchColOpts      = 32
	matchColOFWon     = 34
	matchColOSWon     = 35
	matchColMatchID   = 43
)

// ParseMatchMXArrays maps player-classic matchmx rows into RecentResult values.
// matchmx is listed newest-first; morematchmx holds older career rows. Rows are
// merged as on the TA site (matchmx then morematchmx), deduped by matchid when present.
func ParseMatchMXArrays(matchmx, morematchmx [][]string) ([]models.RecentResult, error) {
	merged := make([][]string, 0, len(matchmx)+len(morematchmx))
	merged = append(merged, matchmx...)
	merged = append(merged, morematchmx...)

	seen := make(map[string]struct{})
	out := make([]models.RecentResult, 0, len(merged))
	for _, row := range merged {
		id := matchCell(row, matchColMatchID)
		if id != "" {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
		}

		rr, err := parseMatchMXRow(row)
		if err != nil {
			continue
		}
		out = append(out, rr)
	}
	return out, nil
}

func parseMatchMXRow(row []string) (models.RecentResult, error) {
	dateStr := matchCell(row, matchColDate)
	if dateStr == "" {
		return models.RecentResult{}, fmt.Errorf("missing date")
	}
	date, err := parseClassicDate(dateStr)
	if err != nil {
		return models.RecentResult{}, err
	}

	score := matchCell(row, matchColScore)
	pts := matchInt(row, matchColPts)
	firsts := matchInt(row, matchColFirsts)

	rr := models.RecentResult{
		Date:           date,
		Tournament:     matchCell(row, matchColTourn),
		Surface:        matchCell(row, matchColSurf),
		Round:          matchCell(row, matchColRound),
		Rank:           matchInt(row, matchColRank),
		OpponentRank:   matchInt(row, matchColORank),
		Score:          score,
		DominanceRatio: classicDominanceRatio(row, score),
		AcePct:         classicRate(matchInt(row, matchColAces), pts),
		DFPct:          classicRate(matchInt(row, matchColDFs), pts),
		FirstServeIn:   classicRate(firsts, pts),
		FirstServeWon:  classicRate(matchInt(row, matchColFWon), firsts),
		SecondServeWon: classicRate(matchInt(row, matchColSWon), pts-firsts),
		BPSaved:        classicBPSaved(row),
		Duration:       classicDuration(matchCell(row, matchColTime)),
	}
	return rr, nil
}

func parseClassicDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if len(s) != 8 {
		return time.Time{}, fmt.Errorf("invalid classic date %q", s)
	}
	return time.ParseInLocation("20060102", s, time.UTC)
}

func matchCell(row []string, col int) string {
	if col < 0 || col >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[col])
}

func matchInt(row []string, col int) int {
	return parseInt(matchCell(row, col))
}

func classicDominanceRatio(row []string, score string) *float64 {
	if isWalkoverScore(score) {
		return nil
	}
	opts := matchInt(row, matchColOpts)
	pts := matchInt(row, matchColPts)
	ofwon := matchCell(row, matchColOFWon)
	oswon := matchCell(row, matchColOSWon)
	fwon := matchCell(row, matchColFWon)
	swon := matchCell(row, matchColSWon)
	if opts <= 0 || pts <= 0 || ofwon == "" || oswon == "" || fwon == "" || swon == "" {
		return nil
	}
	rpw := 1 - float64(matchInt(row, matchColOFWon)+matchInt(row, matchColOSWon))/float64(opts)
	spl := 1 - float64(matchInt(row, matchColFWon)+matchInt(row, matchColSWon))/float64(pts)
	if spl == 0 {
		return nil
	}
	dr := rpw / spl
	return &dr
}

func isWalkoverScore(score string) bool {
	s := strings.TrimSpace(strings.ToUpper(score))
	return s == "W/O" || strings.HasPrefix(s, "W/O ")
}

func classicRate(num, denom int) float64 {
	if denom <= 0 || num < 0 {
		return 0
	}
	return float64(num) / float64(denom)
}

func classicBPSaved(row []string) string {
	saved := matchCell(row, matchColSaved)
	chances := matchCell(row, matchColChances)
	if saved == "" || chances == "" {
		return ""
	}
	if _, err := strconv.Atoi(chances); err != nil || chances == "0" {
		return ""
	}
	return saved + "/" + chances
}

// classicDuration formats TA match.time (total minutes) as H:MM.
func classicDuration(minutesStr string) string {
	minutesStr = strings.TrimSpace(minutesStr)
	if minutesStr == "" {
		return ""
	}
	total, err := strconv.Atoi(minutesStr)
	if err != nil || total < 0 {
		return ""
	}
	h := total / 60
	m := total % 60
	return fmt.Sprintf("%d:%02d", h, m)
}
