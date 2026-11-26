package tennisabstract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return b
}

func medvedevClassicSnippet(t *testing.T) []byte {
	t.Helper()
	return loadTestdata(t, "player_classic_medvedev_snip.html")
}

func medvedevCareerJSSnippet(t *testing.T) []byte {
	t.Helper()
	return loadTestdata(t, "medvedev_career_snip.js")
}

func TestExtractJSArray_matchmx(t *testing.T) {
	t.Parallel()

	rows, err := extractJSArray(medvedevClassicSnippet(t), "matchmx")
	if err != nil {
		t.Fatalf("extractJSArray: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("rows: got %d want 5", len(rows))
	}
	if rows[0][0] != "20260506" || rows[0][1] != "Rome Masters" || rows[0][8] != "SF" {
		t.Fatalf("first row: %#v", rows[0][:10])
	}
	if rows[1][9] != "W/O" || rows[2][9] != "W/O" {
		t.Fatalf("W/O rows: scores=%q, %q", rows[1][9], rows[2][9])
	}
}

func TestExtractJSArray_morematchmx(t *testing.T) {
	t.Parallel()

	rows, err := extractJSArray(medvedevCareerJSSnippet(t), "morematchmx")
	if err != nil {
		t.Fatalf("extractJSArray: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d want 2", len(rows))
	}
	if rows[0][1] != "Russia F1" || rows[1][4] != "W" {
		t.Fatalf("unexpected rows: %#v, %#v", rows[0], rows[1])
	}
}

func TestExtractJSArray_missing(t *testing.T) {
	t.Parallel()

	_, err := extractJSArray([]byte("var other = [];"), "matchmx")
	if err == nil || !strings.Contains(err.Error(), "matchmx assignment not found") {
		t.Fatalf("expected missing assignment error, got %v", err)
	}
}

func TestFetchPlayerClassicPage_httptest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/player-classic.cgi" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("p") != "DaniilMedvedev" || r.URL.Query().Get("f") != "ACareerqq" {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		_, _ = w.Write(medvedevClassicSnippet(t))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	body, err := c.fetchPlayerClassicPage(context.Background(), "DaniilMedvedev", "ACareerqq")
	if err != nil {
		t.Fatalf("fetchPlayerClassicPage: %v", err)
	}
	rows, err := extractJSArray(body, "matchmx")
	if err != nil {
		t.Fatalf("extractJSArray: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("rows: got %d want 5", len(rows))
	}
}

func TestFetchCareerMatchesJS_httptest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/jsmatches/DaniilMedvedevCareer.js":
			_, _ = w.Write(medvedevCareerJSSnippet(t))
		case "/jsmatches/NoCareerFileCareer.js":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))

	body, err := c.fetchCareerMatchesJS(context.Background(), "DaniilMedvedev")
	if err != nil {
		t.Fatalf("fetchCareerMatchesJS: %v", err)
	}
	rows, err := extractJSArray(body, "morematchmx")
	if err != nil {
		t.Fatalf("extractJSArray: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d want 2", len(rows))
	}

	missing, err := c.fetchCareerMatchesJS(context.Background(), "NoCareerFile")
	if err != nil {
		t.Fatalf("fetchCareerMatchesJS missing file: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("expected empty body on 404, got %q", missing)
	}
}

func TestPlayerClassicPath(t *testing.T) {
	t.Parallel()

	got := playerClassicPath("DaniilMedvedev", "")
	if !strings.Contains(got, "p=DaniilMedvedev") || !strings.Contains(got, "f=ACareerqq") {
		t.Fatalf("playerClassicPath: %q", got)
	}
	if careerMatchesPath("DaniilMedvedev") != "/jsmatches/DaniilMedvedevCareer.js" {
		t.Fatalf("careerMatchesPath: %q", careerMatchesPath("DaniilMedvedev"))
	}
}
