package tennisabstract

import (
	"os"
	"strings"
	"testing"
)

func TestExtractPlayerFrag_fixtureWrapped(t *testing.T) {
	t.Parallel()

	html, err := os.ReadFile("testdata/player_medvedev.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	js := []byte(playerFragAssign + string(html) + playerFragEnd)

	got, err := extractPlayerFrag(js)
	if err != nil {
		t.Fatalf("extractPlayerFrag: %v", err)
	}
	if !strings.Contains(string(got), "recent-results-h") {
		t.Fatalf("missing recent-results section: %.80q", got)
	}
}

func TestExtractPlayerFrag_missing(t *testing.T) {
	t.Parallel()

	_, err := extractPlayerFrag([]byte("var other = 1;"))
	if err == nil || !strings.Contains(err.Error(), "player_frag assignment not found") {
		t.Fatalf("expected missing assignment error, got %v", err)
	}
}

func TestExtractPlayerFrag_unterminated(t *testing.T) {
	t.Parallel()

	_, err := extractPlayerFrag([]byte(playerFragAssign + "<p>no end"))
	if err == nil || !strings.Contains(err.Error(), "not terminated") {
		t.Fatalf("expected unterminated error, got %v", err)
	}
}
