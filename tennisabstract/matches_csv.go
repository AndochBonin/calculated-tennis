package tennisabstract

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/AndochBonin/polymarket/tennis"
)

const (
	csvColWinnerName  = "winner_name"
	csvColLoserName   = "loser_name"
	csvColSurface     = "surface"
	csvColScore       = "score"
	csvColBestOf = "best_of"
)

// MatchSurface is a normalized court surface from Sackmann ATP match CSVs.
type MatchSurface string

const (
	SurfaceHard  MatchSurface = "Hard"
	SurfaceClay  MatchSurface = "Clay"
	SurfaceGrass MatchSurface = "Grass"
)

// CalibrationMatch is one completed 2025 ATP match for alpha calibration.
// Player A is the actual winner; player B is the loser.
type CalibrationMatch struct {
	Surface     MatchSurface
	PlayerA     string
	PlayerB     string
	PlayerASlug string
	PlayerBSlug string
	Format      tennis.MatchFormat
	Score       string
	BestOf      int
	TourneyDate int // YYYYMMDD
}

// CalibrationMatchesLoad holds eligible matches grouped by surface and skip counts.
type CalibrationMatchesLoad struct {
	BySurface map[MatchSurface][]CalibrationMatch
	// SkippedIncomplete is rows whose score contains RET, W/O, or DEF.
	SkippedIncomplete int
	// SkippedInvalid is rows with unknown surface, best_of, or missing player names.
	SkippedInvalid int
}

// LoadCalibrationMatchesCSV parses a Sackmann-style ATP matches CSV, drops
// incomplete scores (RET, W/O, DEF), and groups eligible rows by surface.
// Winner is player A; format follows best_of (3 → DefaultFormat, 5 → GrandSlamMenFormat).
func LoadCalibrationMatchesCSV(r io.Reader) (*CalibrationMatchesLoad, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}
	idx, err := calibrationColumnIndices(header)
	if err != nil {
		return nil, err
	}

	out := &CalibrationMatchesLoad{
		BySurface: map[MatchSurface][]CalibrationMatch{
			SurfaceHard:  nil,
			SurfaceClay:  nil,
			SurfaceGrass: nil,
		},
	}

	for {
		row, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read csv row: %w", err)
		}

		m, skipReason, err := parseCalibrationRow(row, idx)
		if err != nil {
			return nil, err
		}
		switch skipReason {
		case skipNone:
			out.BySurface[m.Surface] = append(out.BySurface[m.Surface], m)
		case skipIncomplete:
			out.SkippedIncomplete++
		case skipInvalid:
			out.SkippedInvalid++
		}
	}
	return out, nil
}

// LoadCalibrationMatchesCSVFile is LoadCalibrationMatchesCSV from a file path.
func LoadCalibrationMatchesCSVFile(path string) (*CalibrationMatchesLoad, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadCalibrationMatchesCSV(f)
}

type skipReason int

const (
	skipNone skipReason = iota
	skipIncomplete
	skipInvalid
)

type calibrationColumns struct {
	winnerName  int
	loserName   int
	surface     int
	score       int
	bestOf      int
	tourneyDate int
}

func calibrationColumnIndices(header []string) (calibrationColumns, error) {
	idx := calibrationColumns{
		winnerName:  -1,
		loserName:   -1,
		surface:     -1,
		score:       -1,
		bestOf:      -1,
		tourneyDate: -1,
	}
	set := func(name string, target *int) {
		for i, col := range header {
			if strings.TrimSpace(col) == name {
				*target = i
				return
			}
		}
	}
	set(csvColWinnerName, &idx.winnerName)
	set(csvColLoserName, &idx.loserName)
	set(csvColSurface, &idx.surface)
	set(csvColScore, &idx.score)
	set(csvColBestOf, &idx.bestOf)
	set(csvColTourneyDate, &idx.tourneyDate)

	missing := []string{}
	if idx.winnerName < 0 {
		missing = append(missing, csvColWinnerName)
	}
	if idx.loserName < 0 {
		missing = append(missing, csvColLoserName)
	}
	if idx.surface < 0 {
		missing = append(missing, csvColSurface)
	}
	if idx.score < 0 {
		missing = append(missing, csvColScore)
	}
	if idx.bestOf < 0 {
		missing = append(missing, csvColBestOf)
	}
	if idx.tourneyDate < 0 {
		missing = append(missing, csvColTourneyDate)
	}
	if len(missing) > 0 {
		return calibrationColumns{}, fmt.Errorf("csv missing columns: %s", strings.Join(missing, ", "))
	}
	return idx, nil
}

func parseCalibrationRow(row []string, idx calibrationColumns) (CalibrationMatch, skipReason, error) {
	score := fieldAt(row, idx.score)
	if scoreIsIncomplete(score) {
		return CalibrationMatch{}, skipIncomplete, nil
	}

	surface, ok := normalizeMatchSurface(fieldAt(row, idx.surface))
	if !ok {
		return CalibrationMatch{}, skipInvalid, nil
	}

	bestOfStr := fieldAt(row, idx.bestOf)
	bestOf, err := strconv.Atoi(bestOfStr)
	if err != nil {
		return CalibrationMatch{}, skipInvalid, nil
	}
	format, ok := matchFormatForBestOf(bestOf)
	if !ok {
		return CalibrationMatch{}, skipInvalid, nil
	}

	playerA := fieldAt(row, idx.winnerName)
	playerB := fieldAt(row, idx.loserName)
	if playerA == "" || playerB == "" {
		return CalibrationMatch{}, skipInvalid, nil
	}

	slugA := PlayerSlug(playerA)
	slugB := PlayerSlug(playerB)
	if slugA == "" || slugB == "" {
		return CalibrationMatch{}, skipInvalid, nil
	}

	tourneyDate, ok := parseTourneyDate(fieldAt(row, idx.tourneyDate))
	if !ok {
		return CalibrationMatch{}, skipInvalid, nil
	}

	return CalibrationMatch{
		Surface:     surface,
		PlayerA:     playerA,
		PlayerB:     playerB,
		PlayerASlug: slugA,
		PlayerBSlug: slugB,
		Format:      format,
		Score:       score,
		BestOf:      bestOf,
		TourneyDate: tourneyDate,
	}, skipNone, nil
}

func fieldAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// scoreIsIncomplete reports walkover, retirement, or default scores.
func scoreIsIncomplete(score string) bool {
	if strings.Contains(score, "RET") {
		return true
	}
	if strings.Contains(score, "W/O") {
		return true
	}
	if strings.Contains(score, "DEF") {
		return true
	}
	return false
}

func normalizeMatchSurface(raw string) (MatchSurface, bool) {
	switch strings.TrimSpace(raw) {
	case string(SurfaceHard):
		return SurfaceHard, true
	case string(SurfaceClay):
		return SurfaceClay, true
	case string(SurfaceGrass):
		return SurfaceGrass, true
	default:
		return "", false
	}
}

func matchFormatForBestOf(bestOf int) (tennis.MatchFormat, bool) {
	switch bestOf {
	case 3:
		return tennis.DefaultFormat(), true
	case 5:
		return tennis.GrandSlamMenFormat(), true
	default:
		return tennis.MatchFormat{}, false
	}
}

// UniquePlayerNamesFromMatchesCSV returns sorted distinct display names from
// winner_name and loser_name in a Sackmann-style ATP matches CSV.
func UniquePlayerNamesFromMatchesCSV(r io.Reader) ([]string, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}
	winnerIdx, loserIdx, err := matchNameColumnIndices(header)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	for {
		row, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read csv row: %w", err)
		}
		for _, idx := range []int{winnerIdx, loserIdx} {
			if idx >= len(row) {
				continue
			}
			name := strings.TrimSpace(row[idx])
			if name == "" {
				continue
			}
			seen[name] = struct{}{}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func matchNameColumnIndices(header []string) (winnerIdx, loserIdx int, err error) {
	winnerIdx = -1
	loserIdx = -1
	for i, col := range header {
		switch strings.TrimSpace(col) {
		case csvColWinnerName:
			winnerIdx = i
		case csvColLoserName:
			loserIdx = i
		}
	}
	if winnerIdx < 0 || loserIdx < 0 {
		return 0, 0, fmt.Errorf("csv missing %q and/or %q columns", csvColWinnerName, csvColLoserName)
	}
	return winnerIdx, loserIdx, nil
}
