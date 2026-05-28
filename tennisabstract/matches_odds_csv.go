package tennisabstract

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/AndochBonin/polymarket/tennis"
)

const (
	csvColTourneyDate = "tourney_date"
	csvColMatchNum    = "match_num"
)

// MatchWithOdds is one Sackmann ATP match row with joined decimal odds (AvgW/AvgL).
// Player A is the winner; player B is the loser.
type MatchWithOdds struct {
	Surface     MatchSurface
	PlayerA     string
	PlayerB     string
	PlayerASlug string
	PlayerBSlug string
	Format      tennis.MatchFormat
	Score       string
	BestOf      int
	TourneyDate int // YYYYMMDD
	MatchNum    int
	AvgW        float64
	AvgL        float64
}

// LoadMatchesWithOddsCSV parses a Sackmann ATP matches CSV with trailing AvgW and AvgL.
// Rows with unknown surface, best_of, or missing player names are omitted.
func LoadMatchesWithOddsCSV(r io.Reader) ([]MatchWithOdds, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}
	idx, err := matchWithOddsColumnIndices(header)
	if err != nil {
		return nil, err
	}

	var out []MatchWithOdds
	for {
		row, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read csv row: %w", err)
		}
		m, ok, err := parseMatchWithOddsRow(row, idx)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// LoadMatchesWithOddsCSVFile is LoadMatchesWithOddsCSV from a file path.
func LoadMatchesWithOddsCSVFile(path string) ([]MatchWithOdds, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadMatchesWithOddsCSV(f)
}

// FilterBacktestMatches keeps rows with a complete score, both players in rates
// (2024 hold/break), and parseable AvgW and AvgL.
func FilterBacktestMatches(rows []MatchWithOdds, rates PlayerRatesMap) []MatchWithOdds {
	if len(rows) == 0 {
		return nil
	}
	out := make([]MatchWithOdds, 0, len(rows))
	for _, m := range rows {
		if !backtestMatchEligible(m, rates) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func backtestMatchEligible(m MatchWithOdds, rates PlayerRatesMap) bool {
	if scoreIsIncomplete(m.Score) {
		return false
	}
	if !matchWithOddsHasRates(m, rates) {
		return false
	}
	if m.AvgW <= 0 || m.AvgL <= 0 {
		return false
	}
	return true
}

func matchWithOddsHasRates(m MatchWithOdds, rates PlayerRatesMap) bool {
	_, okA := rates[m.PlayerASlug]
	_, okB := rates[m.PlayerBSlug]
	return okA && okB
}

type matchWithOddsColumns struct {
	calibrationColumns
	tourneyDate int
	matchNum    int
	avgW        int
	avgL        int
}

func matchWithOddsColumnIndices(header []string) (matchWithOddsColumns, error) {
	base, err := calibrationColumnIndices(header)
	if err != nil {
		return matchWithOddsColumns{}, err
	}
	idx := matchWithOddsColumns{
		calibrationColumns: base,
		tourneyDate:        -1,
		matchNum:           -1,
		avgW:               -1,
		avgL:               -1,
	}
	set := func(name string, target *int) {
		for i, col := range header {
			if strings.TrimSpace(col) == name {
				*target = i
				return
			}
		}
	}
	set(csvColTourneyDate, &idx.tourneyDate)
	set(csvColMatchNum, &idx.matchNum)
	set(oddsColAvgW, &idx.avgW)
	set(oddsColAvgL, &idx.avgL)

	missing := []string{}
	if idx.tourneyDate < 0 {
		missing = append(missing, csvColTourneyDate)
	}
	if idx.matchNum < 0 {
		missing = append(missing, csvColMatchNum)
	}
	if idx.avgW < 0 {
		missing = append(missing, oddsColAvgW)
	}
	if idx.avgL < 0 {
		missing = append(missing, oddsColAvgL)
	}
	if len(missing) > 0 {
		return matchWithOddsColumns{}, fmt.Errorf("csv missing columns: %s", strings.Join(missing, ", "))
	}
	return idx, nil
}

func parseMatchWithOddsRow(row []string, idx matchWithOddsColumns) (MatchWithOdds, bool, error) {
	score := fieldAt(row, idx.score)

	surface, ok := normalizeMatchSurface(fieldAt(row, idx.surface))
	if !ok {
		return MatchWithOdds{}, false, nil
	}

	bestOfStr := fieldAt(row, idx.bestOf)
	bestOf, err := strconv.Atoi(bestOfStr)
	if err != nil {
		return MatchWithOdds{}, false, nil
	}
	format, ok := matchFormatForBestOf(bestOf)
	if !ok {
		return MatchWithOdds{}, false, nil
	}

	playerA := fieldAt(row, idx.winnerName)
	playerB := fieldAt(row, idx.loserName)
	if playerA == "" || playerB == "" {
		return MatchWithOdds{}, false, nil
	}

	slugA := PlayerSlug(playerA)
	slugB := PlayerSlug(playerB)
	if slugA == "" || slugB == "" {
		return MatchWithOdds{}, false, nil
	}

	tourneyDate, ok := parseTourneyDate(fieldAt(row, idx.tourneyDate))
	if !ok {
		return MatchWithOdds{}, false, nil
	}

	matchNum, err := strconv.Atoi(fieldAt(row, idx.matchNum))
	if err != nil {
		return MatchWithOdds{}, false, nil
	}

	avgW := parseOptionalFloat(fieldAt(row, idx.avgW))
	avgL := parseOptionalFloat(fieldAt(row, idx.avgL))

	m := MatchWithOdds{
		Surface:     surface,
		PlayerA:     playerA,
		PlayerB:     playerB,
		PlayerASlug: slugA,
		PlayerBSlug: slugB,
		Format:      format,
		Score:       score,
		BestOf:      bestOf,
		TourneyDate: tourneyDate,
		MatchNum:    matchNum,
	}
	if avgW != nil {
		m.AvgW = *avgW
	}
	if avgL != nil {
		m.AvgL = *avgL
	}
	return m, true, nil
}

func parseTourneyDate(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 8 {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}
