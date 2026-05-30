package core

import "testing"

func TestAppendVerboseIDs(t *testing.T) {
	t.Run("returns nil when disabled", func(t *testing.T) {
		prev := VerboseIDs
		VerboseIDs = false
		t.Cleanup(func() { VerboseIDs = prev })

		if got := AppendVerboseIDs("token_id", "abc"); got != nil {
			t.Fatalf("expected nil when VerboseIDs=false, got %#v", got)
		}
	})

	t.Run("returns kv pairs when enabled", func(t *testing.T) {
		prev := VerboseIDs
		VerboseIDs = true
		t.Cleanup(func() { VerboseIDs = prev })

		got := AppendVerboseIDs("token_id", "abc", "name", "match")
		if len(got) != 4 {
			t.Fatalf("expected 4 entries, got %d", len(got))
		}
		if got[0] != "token_id" || got[1] != "abc" || got[2] != "name" || got[3] != "match" {
			t.Fatalf("unexpected kv payload: %#v", got)
		}
	})
}
