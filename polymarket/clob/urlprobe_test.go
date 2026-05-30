package clob

import (
	"net/http"
	"net/url"
	"testing"
)

func TestProbeJoinPathParseNewRequestEdgeCases(t *testing.T) {
	t.Skip("manual probe — delete after finding edge cases")
	bases := []string{
		"http://example.com",
		"http://example.com:8080",
		"https://example.com/foo/bar",
		"http://[::1]:8080",
		"http://example.com/foo%00bar",
	}
	for _, b := range bases {
		s, err := url.JoinPath(b, "positions")
		if err != nil {
			t.Logf("JoinPath err %q: %v", b, err)
			continue
		}
		if _, e2 := url.Parse(s); e2 != nil {
			t.Fatalf("Parse2 fail %q -> %q: %v", b, s, e2)
		}
		if _, e3 := http.NewRequest(http.MethodGet, s, nil); e3 != nil {
			t.Fatalf("NewRequest fail %q -> %q: %v", b, s, e3)
		}
	}
}
