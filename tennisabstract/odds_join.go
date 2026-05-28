package tennisabstract

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	oddsColDate    = "Date"
	oddsColSurface = "Surface"
	oddsColWinner  = "Winner"
	oddsColLoser   = "Loser"
	oddsColComment = "Comment"
	oddsColAvgW    = "AvgW"
	oddsColAvgL    = "AvgL"

	sackColTourneyDate = "tourney_date"

	// MatchJoinMaxDateDiff is the max calendar-day gap between Sackmann tourney_date
	// (tournament start) and tennis-data match Date when joining odds. Slams and long
	// events can schedule matches two weeks after tourney_date.
	MatchJoinMaxDateDiff = 21
)

// OddsRow is one tennis-data odds CSV row used for joins.
type OddsRow struct {
	Date    time.Time
	Surface string
	Winner  string
	Loser   string
	Comment string
	AvgW    *float64
	AvgL    *float64
}

type oddsMatchKey struct {
	winner playerMatchKey
	loser  playerMatchKey
	surface string
}

// LoadOddsCSV reads a tennis-data style odds CSV.
func LoadOddsCSV(r io.Reader) ([]OddsRow, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read odds header: %w", err)
	}
	idx, err := oddsColumnIndices(header)
	if err != nil {
		return nil, err
	}

	var rows []OddsRow
	for {
		rec, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read odds row: %w", err)
		}
		row, err := parseOddsRow(rec, idx)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

type oddsColumns struct {
	date, surface, winner, loser, comment, avgW, avgL int
}

func oddsColumnIndices(header []string) (oddsColumns, error) {
	var idx oddsColumns
	set := func(name string, target *int) {
		for i, col := range header {
			if strings.TrimSpace(col) == name {
				*target = i
				return
			}
		}
	}
	set(oddsColDate, &idx.date)
	set(oddsColSurface, &idx.surface)
	set(oddsColWinner, &idx.winner)
	set(oddsColLoser, &idx.loser)
	set(oddsColComment, &idx.comment)
	set(oddsColAvgW, &idx.avgW)
	set(oddsColAvgL, &idx.avgL)

	missing := []string{}
	if idx.date < 0 {
		missing = append(missing, oddsColDate)
	}
	if idx.surface < 0 {
		missing = append(missing, oddsColSurface)
	}
	if idx.winner < 0 {
		missing = append(missing, oddsColWinner)
	}
	if idx.loser < 0 {
		missing = append(missing, oddsColLoser)
	}
	if idx.avgW < 0 {
		missing = append(missing, oddsColAvgW)
	}
	if idx.avgL < 0 {
		missing = append(missing, oddsColAvgL)
	}
	if len(missing) > 0 {
		return oddsColumns{}, fmt.Errorf("odds csv missing columns: %s", strings.Join(missing, ", "))
	}
	return idx, nil
}

func parseOddsRow(rec []string, idx oddsColumns) (OddsRow, error) {
	dateStr := fieldAt(rec, idx.date)
	t, err := time.Parse("1/2/2006", dateStr)
	if err != nil {
		return OddsRow{}, fmt.Errorf("parse date %q: %w", dateStr, err)
	}
	avgW := parseOptionalFloat(fieldAt(rec, idx.avgW))
	avgL := parseOptionalFloat(fieldAt(rec, idx.avgL))
	comment := ""
	if idx.comment >= 0 {
		comment = fieldAt(rec, idx.comment)
	}
	return OddsRow{
		Date:    t,
		Surface: fieldAt(rec, idx.surface),
		Winner:  fieldAt(rec, idx.winner),
		Loser:   fieldAt(rec, idx.loser),
		Comment: comment,
		AvgW:    avgW,
		AvgL:    avgL,
	}, nil
}

// JoinMatchesOddsResult reports join statistics from JoinMatchesWithAvgOddsCSV.
type JoinMatchesOddsResult struct {
	RowsWritten   int
	RowsMatched   int
	RowsUnmatched int
}

// JoinMatchesWithAvgOddsCSV writes all Sackmann match rows plus AvgW and AvgL columns.
// Odds are matched on winner, loser, surface, and date within MatchJoinMaxDateDiff days.
func JoinMatchesWithAvgOddsCSV(matchesPath, oddsPath, outPath string) (JoinMatchesOddsResult, error) {
	mf, err := os.Open(matchesPath)
	if err != nil {
		return JoinMatchesOddsResult{}, err
	}
	defer mf.Close()

	of, err := os.Open(oddsPath)
	if err != nil {
		return JoinMatchesOddsResult{}, err
	}
	defer of.Close()

	oddsRows, err := LoadOddsCSV(of)
	if err != nil {
		return JoinMatchesOddsResult{}, fmt.Errorf("load odds: %w", err)
	}
	index := buildOddsIndex(oddsRows)

	cr := csv.NewReader(mf)
	cr.ReuseRecord = true
	header, err := cr.Read()
	if err != nil {
		return JoinMatchesOddsResult{}, fmt.Errorf("read matches header: %w", err)
	}
	colIdx, err := calibrationColumnIndices(header)
	if err != nil {
		return JoinMatchesOddsResult{}, err
	}
	tourneyDateIdx := -1
	for i, col := range header {
		if strings.TrimSpace(col) == sackColTourneyDate {
			tourneyDateIdx = i
			break
		}
	}
	if tourneyDateIdx < 0 {
		return JoinMatchesOddsResult{}, fmt.Errorf("matches csv missing %q column", sackColTourneyDate)
	}

	outHeader := append(append([]string(nil), header...), oddsColAvgW, oddsColAvgL)
	var stats JoinMatchesOddsResult

	outF, err := os.Create(outPath)
	if err != nil {
		return JoinMatchesOddsResult{}, err
	}
	defer outF.Close()

	w := csv.NewWriter(outF)
	if err := w.Write(outHeader); err != nil {
		return JoinMatchesOddsResult{}, err
	}

	for {
		row, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return JoinMatchesOddsResult{}, fmt.Errorf("read matches row: %w", err)
		}
		stats.RowsWritten++

		avgW, avgL := "", ""
		if match, ok := parseMatchRowForOddsJoin(row, colIdx, tourneyDateIdx); ok {
			if o := findBestOddsMatch(index, match); o != nil {
				if o.AvgW != nil {
					avgW = formatOdds(*o.AvgW)
				}
				if o.AvgL != nil {
					avgL = formatOdds(*o.AvgL)
				}
				if avgW != "" && avgL != "" {
					stats.RowsMatched++
				}
			}
		}
		if avgW == "" || avgL == "" {
			stats.RowsUnmatched++
		}

		outRow := append(append([]string(nil), row...), avgW, avgL)
		if err := w.Write(outRow); err != nil {
			return JoinMatchesOddsResult{}, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return JoinMatchesOddsResult{}, err
	}
	if err := outF.Close(); err != nil {
		return JoinMatchesOddsResult{}, err
	}
	return stats, nil
}

func formatOdds(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

type matchOddsJoin struct {
	tourneyDate time.Time
	winner      string
	loser       string
	surface     string
}

func parseMatchRowForOddsJoin(row []string, idx calibrationColumns, tourneyDateIdx int) (matchOddsJoin, bool) {
	dateStr := fieldAt(row, tourneyDateIdx)
	if len(dateStr) != 8 {
		return matchOddsJoin{}, false
	}
	y, err1 := strconv.Atoi(dateStr[0:4])
	m, err2 := strconv.Atoi(dateStr[4:6])
	d, err3 := strconv.Atoi(dateStr[6:8])
	if err1 != nil || err2 != nil || err3 != nil {
		return matchOddsJoin{}, false
	}
	tourneyDate := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)

	winner := fieldAt(row, idx.winnerName)
	loser := fieldAt(row, idx.loserName)
	surface := fieldAt(row, idx.surface)
	if winner == "" || loser == "" || surface == "" {
		return matchOddsJoin{}, false
	}
	return matchOddsJoin{
		tourneyDate: tourneyDate,
		winner:      winner,
		loser:       loser,
		surface:     surface,
	}, true
}

type indexedOdds struct {
	rows []OddsRow
}

func buildOddsIndex(odds []OddsRow) map[oddsMatchKey]indexedOdds {
	index := make(map[oddsMatchKey]indexedOdds)
	for _, o := range odds {
		if o.AvgW == nil || o.AvgL == nil {
			continue
		}
		if strings.TrimSpace(o.Comment) != "" && o.Comment != "Completed" {
			continue
		}
		k := oddsMatchKey{
			winner:  playerKeyFromOddsName(o.Winner),
			loser:   playerKeyFromOddsName(o.Loser),
			surface: strings.TrimSpace(o.Surface),
		}
		bucket := index[k]
		bucket.rows = append(bucket.rows, o)
		index[k] = bucket
	}
	return index
}

func findBestOddsMatch(index map[oddsMatchKey]indexedOdds, m matchOddsJoin) *OddsRow {
	k := oddsMatchKey{
		winner:  playerKeyFromSackmannName(m.winner),
		loser:   playerKeyFromSackmannName(m.loser),
		surface: strings.TrimSpace(m.surface),
	}
	bucket, ok := index[k]
	if !ok || len(bucket.rows) == 0 {
		return nil
	}

	var best *OddsRow
	bestDiff := MatchJoinMaxDateDiff + 1
	for i := range bucket.rows {
		o := &bucket.rows[i]
		diff := daysApart(m.tourneyDate, o.Date)
		if diff > MatchJoinMaxDateDiff {
			continue
		}
		if best == nil || diff < bestDiff {
			best = o
			bestDiff = diff
		}
	}
	return best
}

func daysApart(a, b time.Time) int {
	a = time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	b = time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return int(d.Hours() / 24)
}
