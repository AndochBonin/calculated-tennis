package tennisabstract

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/AndochBonin/polymarket/models"
	"github.com/PuerkitoBio/goquery"
)

const (
	sectionRecentResults    = "Recent Results"
	sectionTourLevelSeasons = "Tour-Level Seasons"
)

var recentResultsColumns = []string{
	"Date", "Tournament", "Surface", "Rd", "Rk", "vRk", "Score", "DR",
	"A%", "DF%", "1stIn", "1st%", "2nd%", "BPSvd", "Time",
}

var tourLevelColumns = []string{
	"Year", "M", "W", "L", "Win%", "Set W-L", "Set%", "Game W-L", "Game%",
	"TB W-L", "TB%", "MS", "Hld%", "Brk%", "A%", "DF%", "1stIn", "1st%", "2nd%",
	"SPW", "RPW", "TPW", "DR", "Best",
}

// ParsePlayerHTML extracts Recent Results and Tour-Level Seasons from a Tennis
// Abstract player page (or HTML fragment containing those sections).
func ParsePlayerHTML(r io.Reader, playerSlug string) (models.PlayerStats, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return models.PlayerStats{}, fmt.Errorf("parse html: %w", err)
	}

	recent, err := parseRecentResultsTable(doc)
	if err != nil {
		return models.PlayerStats{}, err
	}
	seasons, err := parseTourLevelSeasonsTable(doc)
	if err != nil {
		return models.PlayerStats{}, err
	}

	return models.PlayerStats{
		PlayerSlug:       playerSlug,
		RecentResults:    recent,
		TourLevelSeasons: seasons,
	}, nil
}

func parseRecentResultsTable(doc *goquery.Document) ([]models.RecentResult, error) {
	table, err := tableAfterHeading(doc, sectionRecentResults)
	if err != nil {
		return nil, fmt.Errorf("recent results: %w", err)
	}
	cols, err := headerIndex(table, recentResultsColumns)
	if err != nil {
		return nil, fmt.Errorf("recent results: %w", err)
	}

	var rows []models.RecentResult
	table.Find("tbody tr").Each(func(_ int, tr *goquery.Selection) {
		cells := rowCells(tr)
		if len(cells) == 0 {
			return
		}
		dateStr := cellText(cells, cols["Date"])
		if dateStr == "" {
			return
		}
		date, err := parseTennisAbstractDate(dateStr)
		if err != nil {
			return
		}

		tournament, tournamentURL := linkTextAndHref(cells, cols["Tournament"])
		row := models.RecentResult{
			Date:           date,
			Tournament:     tournament,
			TournamentURL:  tournamentURL,
			Surface:        cellText(cells, cols["Surface"]),
			Round:          cellText(cells, cols["Rd"]),
			Rank:           parseInt(cellText(cells, cols["Rk"])),
			OpponentRank:   parseInt(cellText(cells, cols["vRk"])),
			Score:          cellText(cells, cols["Score"]),
			DominanceRatio: parseOptionalFloat(cellText(cells, cols["DR"])),
			AcePct:         parsePercent(cellText(cells, cols["A%"])),
			DFPct:          parsePercent(cellText(cells, cols["DF%"])),
			FirstServeIn:   parsePercent(cellText(cells, cols["1stIn"])),
			FirstServeWon:  parsePercent(cellText(cells, cols["1st%"])),
			SecondServeWon: parsePercent(cellText(cells, cols["2nd%"])),
			BPSaved:        cellText(cells, cols["BPSvd"]),
			Duration:       cellText(cells, cols["Time"]),
		}
		rows = append(rows, row)
	})
	return rows, nil
}

func parseTourLevelSeasonsTable(doc *goquery.Document) ([]models.TourLevelSeason, error) {
	table, err := tableAfterHeading(doc, sectionTourLevelSeasons)
	if err != nil {
		return nil, fmt.Errorf("tour-level seasons: %w", err)
	}
	cols, err := headerIndex(table, tourLevelColumns)
	if err != nil {
		return nil, fmt.Errorf("tour-level seasons: %w", err)
	}

	var rows []models.TourLevelSeason
	table.Find("tbody tr").Each(func(_ int, tr *goquery.Selection) {
		cells := rowCells(tr)
		if len(cells) == 0 {
			return
		}
		yearCell := cells[cols["Year"]]
		yearText := strings.TrimSpace(yearCell.Text())
		if yearText == "" {
			return
		}

		row := models.TourLevelSeason{
			Matches:        parseInt(cellText(cells, cols["M"])),
			Wins:           parseInt(cellText(cells, cols["W"])),
			Losses:         parseInt(cellText(cells, cols["L"])),
			WinPct:         parsePercent(cellText(cells, cols["Win%"])),
			SetWL:          cellText(cells, cols["Set W-L"]),
			SetPct:         parsePercent(cellText(cells, cols["Set%"])),
			GameWL:         cellText(cells, cols["Game W-L"]),
			GamePct:        parsePercent(cellText(cells, cols["Game%"])),
			TiebreakWL:     cellText(cells, cols["TB W-L"]),
			TiebreakPct:    parsePercent(cellText(cells, cols["TB%"])),
			MatchStats:     parseInt(cellText(cells, cols["MS"])),
			HoldPct:        parsePercent(cellText(cells, cols["Hld%"])),
			BreakPct:       parsePercent(cellText(cells, cols["Brk%"])),
			AcePct:         parsePercent(cellText(cells, cols["A%"])),
			DFPct:          parsePercent(cellText(cells, cols["DF%"])),
			FirstServeIn:   parsePercent(cellText(cells, cols["1stIn"])),
			FirstServeWon:  parsePercent(cellText(cells, cols["1st%"])),
			SecondServeWon: parsePercent(cellText(cells, cols["2nd%"])),
			SPW:            parsePercent(cellText(cells, cols["SPW"])),
			RPW:            parsePercent(cellText(cells, cols["RPW"])),
			TPW:            parsePercent(cellText(cells, cols["TPW"])),
			DR:             parseFloat(cellText(cells, cols["DR"])),
			Best:           strings.TrimSpace(cellText(cells, cols["Best"])),
		}

		if strings.EqualFold(yearText, "Career") {
			row.IsCareer = true
		} else {
			row.Year = parseInt(yearText)
			row.YearURL, _ = yearCell.Find("a").Attr("href")
		}
		rows = append(rows, row)
	})
	return rows, nil
}

func tableAfterHeading(doc *goquery.Document, title string) (*goquery.Selection, error) {
	var heading *goquery.Selection
	doc.Find("h1, h2, h3").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if strings.Contains(normalizeHeading(s.Text()), title) {
			heading = s
			return false
		}
		return true
	})
	if heading == nil || heading.Length() == 0 {
		return nil, fmt.Errorf("%w: heading %q", ErrTableNotFound, title)
	}

	var table *goquery.Selection
	heading.NextAllFiltered("table").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		table = s
		return false
	})
	if table == nil || table.Length() == 0 {
		return nil, fmt.Errorf("%w: after heading %q", ErrTableNotFound, title)
	}
	return table, nil
}

func normalizeHeading(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "\n"); idx >= 0 {
		s = s[:idx]
	}
	return s
}

func headerIndex(table *goquery.Selection, required []string) (map[string]int, error) {
	headerRow := table.Find("thead tr").First()
	if headerRow.Length() == 0 {
		headerRow = table.Find("tr").First()
	}
	if headerRow.Length() == 0 {
		return nil, fmt.Errorf("%w: missing header row", ErrUnexpectedColumns)
	}

	index := make(map[string]int)
	headerRow.Find("th, td").Each(func(i int, th *goquery.Selection) {
		name := strings.TrimSpace(th.Text())
		if name == "" {
			return
		}
		if _, ok := index[name]; !ok {
			index[name] = i
		}
	})

	var missing []string
	for _, col := range required {
		if _, ok := index[col]; !ok {
			missing = append(missing, col)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: missing %v", ErrUnexpectedColumns, missing)
	}
	return index, nil
}

func rowCells(tr *goquery.Selection) []*goquery.Selection {
	var cells []*goquery.Selection
	tr.Find("td").Each(func(_ int, td *goquery.Selection) {
		cells = append(cells, td)
	})
	return cells
}

func cellText(cells []*goquery.Selection, idx int) string {
	if idx < 0 || idx >= len(cells) {
		return ""
	}
	return strings.TrimSpace(cells[idx].Text())
}

func linkTextAndHref(cells []*goquery.Selection, idx int) (text, href string) {
	if idx < 0 || idx >= len(cells) {
		return "", ""
	}
	cell := cells[idx]
	a := cell.Find("a").First()
	if a.Length() == 0 {
		return strings.TrimSpace(cell.Text()), ""
	}
	href, _ = a.Attr("href")
	return strings.TrimSpace(a.Text()), href
}

func parseTennisAbstractDate(s string) (time.Time, error) {
	return time.Parse("02-Jan-2006", strings.TrimSpace(s))
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func parsePercent(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	if strings.HasSuffix(s, "%") {
		f, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err != nil {
			return 0
		}
		return f / 100
	}
	return parseFloat(s)
}

func parseOptionalFloat(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return nil
	}
	v := parseFloat(s)
	return &v
}
