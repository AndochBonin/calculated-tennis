package tennisabstract

import (
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func TestParsePlayerHTML_MedvedevFixture(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/player_medvedev.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	stats, err := ParsePlayerHTML(f, "DaniilMedvedev")
	if err != nil {
		t.Fatalf("ParsePlayerHTML: %v", err)
	}
	if stats.PlayerSlug != "DaniilMedvedev" {
		t.Fatalf("PlayerSlug = %q, want DaniilMedvedev", stats.PlayerSlug)
	}
	if len(stats.RecentResults) != 22 {
		t.Fatalf("RecentResults len = %d, want 22", len(stats.RecentResults))
	}
	if len(stats.TourLevelSeasons) != 12 {
		t.Fatalf("TourLevelSeasons len = %d, want 12", len(stats.TourLevelSeasons))
	}

	first := stats.RecentResults[0]
	wantDate := time.Date(2026, time.May, 6, 0, 0, 0, 0, time.UTC)
	if !first.Date.Equal(wantDate) {
		t.Fatalf("first result date = %v, want %v", first.Date, wantDate)
	}
	if first.Tournament != "Rome Masters" {
		t.Fatalf("Tournament = %q, want Rome Masters", first.Tournament)
	}
	if first.TournamentURL != "https://www.tennisabstract.com/cgi-bin/tourney.cgi?t=2026-0416/Rome-Masters" {
		t.Fatalf("TournamentURL = %q", first.TournamentURL)
	}
	if first.Surface != "Clay" {
		t.Fatalf("Surface = %q, want Clay", first.Surface)
	}
	if first.Round != "SF" {
		t.Fatalf("Round = %q, want SF", first.Round)
	}
	if first.Rank != 9 || first.OpponentRank != 1 {
		t.Fatalf("Rk/vRk = %d/%d, want 9/1", first.Rank, first.OpponentRank)
	}
	if first.Score != "6-2 5-7 6-4" {
		t.Fatalf("Score = %q", first.Score)
	}
	if first.DominanceRatio == nil || math.Abs(*first.DominanceRatio-0.73) > 1e-9 {
		t.Fatalf("DR = %v, want 0.73", first.DominanceRatio)
	}
	if math.Abs(first.AcePct-0.038) > 1e-9 {
		t.Fatalf("AcePct = %v, want 0.038", first.AcePct)
	}
	if first.BPSaved != "6/10" {
		t.Fatalf("BPSaved = %q, want 6/10", first.BPSaved)
	}
	if first.Duration != "2:37" {
		t.Fatalf("Duration = %q, want 2:37", first.Duration)
	}

	var season2026, career *struct {
		year int
		w, l int
		win  float64
	}
	for i := range stats.TourLevelSeasons {
		s := stats.TourLevelSeasons[i]
		if s.Year == 2026 && !s.IsCareer {
			season2026 = &struct {
				year int
				w, l int
				win  float64
			}{s.Year, s.Wins, s.Losses, s.WinPct}
			if s.Matches != 34 {
				t.Fatalf("2026 Matches = %d, want 34", s.Matches)
			}
			if s.SetWL != "55-25" {
				t.Fatalf("2026 SetWL = %q, want 55-25", s.SetWL)
			}
			if math.Abs(s.DR-1.18) > 1e-9 {
				t.Fatalf("2026 DR = %v, want 1.18", s.DR)
			}
			if s.YearURL == "" {
				t.Fatal("2026 YearURL is empty")
			}
		}
		if s.IsCareer {
			career = &struct {
				year int
				w, l int
				win  float64
			}{0, s.Wins, s.Losses, s.WinPct}
		}
	}
	if season2026 == nil {
		t.Fatal("2026 season row not found")
	}
	if season2026.w != 26 || season2026.l != 8 {
		t.Fatalf("2026 W/L = %d/%d, want 26/8", season2026.w, season2026.l)
	}
	if math.Abs(season2026.win-0.765) > 1e-9 {
		t.Fatalf("2026 Win%% = %v, want 0.765", season2026.win)
	}
	if career == nil {
		t.Fatal("Career row not found")
	}
	if career.w != 448 || career.l != 186 {
		t.Fatalf("Career W/L = %d/%d, want 448/186", career.w, career.l)
	}
}

func TestParsePlayerHTML_missingSection(t *testing.T) {
	t.Parallel()

	_, err := ParsePlayerHTML(strings.NewReader(`<html><body><h1>Other</h1></body></html>`), "X")
	if err == nil {
		t.Fatal("expected error for missing tables")
	}
	if !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("expected ErrTableNotFound, got %v", err)
	}
}

func TestParsePlayerHTML_headingWithoutTable(t *testing.T) {
	t.Parallel()

	html := `<html><body>
<h2>Recent Results</h2>
<p>no table here</p>
</body></html>`
	_, err := ParsePlayerHTML(strings.NewReader(html), "X")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("expected ErrTableNotFound, got %v", err)
	}
}

func TestParsePlayerHTML_seasonsTableMissing(t *testing.T) {
	t.Parallel()

	recentHeader := strings.Join([]string{
		"Date", "Tournament", "Surface", "Rd", "Rk", "vRk", "Score", "DR",
		"A%", "DF%", "1stIn", "1st%", "2nd%", "BPSvd", "Time",
	}, "</th><th>")

	html := `<html><body>
<h2>Recent Results</h2>
<table><thead><tr><th>` + recentHeader + `</th></tr></thead><tbody></tbody></table>
<h2>Tour-Level Seasons</h2>
<p>no table</p>
</body></html>`

	_, err := ParsePlayerHTML(strings.NewReader(html), "X")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("expected ErrTableNotFound for seasons, got %v", err)
	}
}

func TestParsePlayerHTML_unexpectedColumns(t *testing.T) {
	t.Parallel()

	html := `<html><body>
<h2>Recent Results</h2>
<table><thead><tr><th>Wrong</th></tr></thead><tbody></tbody></table>
<h2>Tour-Level Seasons</h2>
<table><thead><tr><th>Wrong</th></tr></thead><tbody></tbody></table>
</body></html>`
	_, err := ParsePlayerHTML(strings.NewReader(html), "X")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnexpectedColumns) {
		t.Fatalf("expected ErrUnexpectedColumns, got %v", err)
	}
}

func TestParsePlayerHTML_tourLevelUnexpectedColumns(t *testing.T) {
	t.Parallel()

	recentHeader := strings.Join([]string{
		"Date", "Tournament", "Surface", "Rd", "Rk", "vRk", "Score", "DR",
		"A%", "DF%", "1stIn", "1st%", "2nd%", "BPSvd", "Time",
	}, "</th><th>")

	html := `<html><body>
<h2>Recent Results</h2>
<table><thead><tr><th>` + recentHeader + `</th></tr></thead><tbody></tbody></table>
<h2>Tour-Level Seasons</h2>
<table><thead><tr><th>Wrong</th></tr></thead><tbody></tbody></table>
</body></html>`

	_, err := ParsePlayerHTML(strings.NewReader(html), "X")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnexpectedColumns) {
		t.Fatalf("expected ErrUnexpectedColumns, got %v", err)
	}
}

func TestParsePlayerHTML_skipsBadRows(t *testing.T) {
	t.Parallel()

	recentHeader := strings.Join([]string{
		"Date", "Tournament", "Surface", "Rd", "Rk", "vRk", "Score", "DR",
		"A%", "DF%", "1stIn", "1st%", "2nd%", "BPSvd", "Time",
	}, "</th><th>")
	seasonHeader := strings.Join([]string{
		"Year", "M", "W", "L", "Win%", "Set W-L", "Set%", "Game W-L", "Game%",
		"TB W-L", "TB%", "MS", "Hld%", "Brk%", "A%", "DF%", "1stIn", "1st%", "2nd%",
		"SPW", "RPW", "TPW", "DR", "Best",
	}, "</th><th>")

	html := `<html><body>
<h2>Recent Results</h2>
<table><thead><tr><th>` + recentHeader + `</th></tr></thead><tbody>
<tr></tr>
<tr><td></td><td>Skip Me</td><td>Hard</td><td>R32</td><td></td><td></td><td>6-0</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td></td><td></td></tr>
<tr><td>not-a-date</td><td>Plain Tourney</td><td>Hard</td><td>R32</td><td></td><td></td><td>6-0</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td></td><td></td></tr>
<tr><td>06-May-2026</td><td><a href="/t">Linked</a></td><td>Clay</td><td>F</td><td>1</td><td>2</td><td>6-4</td><td>-</td><td>10%</td><td>bad</td><td>notpct</td><td>50%</td><td>40%</td><td>1/1</td><td>1:00</td></tr>
</tbody></table>
<h2>Tour-Level Seasons</h2>
<table><tr><th>` + seasonHeader + `</th></tr><tbody>
<tr></tr>
<tr><td></td><td>0</td><td>0</td><td>0</td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td><td></td></tr>
<tr><td><a href="/year/2026">2026</a></td><td>5</td><td>3</td><td>2</td><td>60%</td><td>1-1</td><td>50%</td><td>10-10</td><td>50%</td><td>0-0</td><td>0%</td><td>1</td><td>80%</td><td>20%</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>1.0</td><td>F</td></tr>
<tr><td>Career</td><td>5</td><td>3</td><td>2</td><td>60%</td><td>1-1</td><td>50%</td><td>10-10</td><td>50%</td><td>0-0</td><td>0%</td><td>1</td><td>80%</td><td>20%</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>1.0</td><td>F</td></tr>
</tbody></table>
</body></html>`

	stats, err := ParsePlayerHTML(strings.NewReader(html), "Test")
	if err != nil {
		t.Fatalf("ParsePlayerHTML: %v", err)
	}
	if len(stats.RecentResults) != 1 {
		t.Fatalf("RecentResults len = %d, want 1 (bad date row skipped)", len(stats.RecentResults))
	}
	row := stats.RecentResults[0]
	if row.Tournament != "Linked" || row.TournamentURL != "/t" {
		t.Fatalf("tournament link = %q %q", row.Tournament, row.TournamentURL)
	}
	if row.DominanceRatio != nil {
		t.Fatal("expected nil DR for dash")
	}
	if len(stats.TourLevelSeasons) != 2 {
		t.Fatalf("TourLevelSeasons len = %d, want 2", len(stats.TourLevelSeasons))
	}
	var year2026 *struct{ url string }
	for i := range stats.TourLevelSeasons {
		s := stats.TourLevelSeasons[i]
		if s.Year == 2026 {
			year2026 = &struct{ url string }{s.YearURL}
		}
	}
	if year2026 == nil || year2026.url != "/year/2026" {
		t.Fatalf("2026 YearURL = %v", year2026)
	}
}

func TestHeaderIndex_skipsEmptyHeaderNames(t *testing.T) {
	t.Parallel()

	html := `<table><thead><tr><th></th><th>Date</th></tr></thead></table>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("goquery: %v", err)
	}
	_, err = headerIndex(doc.Find("table"), []string{"Date"})
	if err != nil {
		t.Fatalf("headerIndex: %v", err)
	}
}

func TestHeaderIndex_duplicateColumns(t *testing.T) {
	t.Parallel()

	html := `<table><thead><tr><th>Date</th><th>Date</th><th>Tournament</th></tr></thead></table>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("goquery: %v", err)
	}
	idx, err := headerIndex(doc.Find("table"), []string{"Date", "Tournament"})
	if err != nil {
		t.Fatalf("headerIndex: %v", err)
	}
	if idx["Date"] != 0 {
		t.Fatalf("Date index = %d, want 0 (first wins)", idx["Date"])
	}
}

func TestHeaderIndex_emptyHeaderRow(t *testing.T) {
	t.Parallel()

	html := `<table></table>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("goquery: %v", err)
	}
	_, err = headerIndex(doc.Find("table"), []string{"Date"})
	if err == nil || !errors.Is(err, ErrUnexpectedColumns) {
		t.Fatalf("expected ErrUnexpectedColumns, got %v", err)
	}
}

func TestCellTextAndLink_outOfRange(t *testing.T) {
	t.Parallel()
	if cellText(nil, 0) != "" {
		t.Fatal("cellText nil")
	}
	if cellText([]*goquery.Selection{}, 1) != "" {
		t.Fatal("cellText short row")
	}
	text, href := linkTextAndHref(nil, 0)
	if text != "" || href != "" {
		t.Fatal("linkTextAndHref nil")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<html><body><table><tr><td>Plain</td></tr></table></body></html>`))
	if err != nil {
		t.Fatalf("goquery: %v", err)
	}
	cell := doc.Find("td")
	text, href = linkTextAndHref([]*goquery.Selection{cell}, 0)
	if text != "Plain" || href != "" {
		t.Fatalf("plain cell = %q %q", text, href)
	}
}
