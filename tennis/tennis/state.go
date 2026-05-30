package tennis

// Phase is whether the current set is in regular games or a tiebreak.
type Phase int

const (
	Regular Phase = iota
	Tiebreak
)

// SetResult records final game totals for a completed set (e.g. 6–4 or 7–6).
type SetResult struct {
	GamesA int
	GamesB int
}

// Set holds in-progress set state.
type Set struct {
	Games                 [2]int
	Phase                 Phase
	TiebreakPoints        [2]int
	PointsPlayedInTB      int
	FirstServerOfTiebreak Player // who serves TB point 1; set when entering tiebreak
}

// Match is the full match state for rules-only advancement (caller picks winners).
type Match struct {
	Format        MatchFormat
	FirstServer   Player
	setsWon       [2]int
	CompletedSets []SetResult
	Current       Set
	server        Player // who serves the next regular game or tiebreak point
	Done          bool
	Winner        *Player
}

// NewMatch starts a match; firstServer serves game 1 of set 1.
// If format is omitted, DefaultFormat is used.
func NewMatch(firstServer Player, format ...MatchFormat) *Match {
	f := DefaultFormat()
	if len(format) > 0 {
		f = format[0]
	}
	return &Match{
		Format:      f,
		FirstServer: firstServer,
		server:      firstServer,
	}
}

// Clone returns a deep copy of m. CompletedSets is copied into a new slice;
// Winner, when set, points at an independent Player value.
func (m *Match) Clone() Match {
	c := Match{
		Format:      m.Format,
		FirstServer: m.FirstServer,
		setsWon:     m.setsWon,
		Current:     m.Current,
		server:      m.server,
		Done:        m.Done,
	}
	if len(m.CompletedSets) > 0 {
		c.CompletedSets = make([]SetResult, len(m.CompletedSets))
		copy(c.CompletedSets, m.CompletedSets)
	}
	if m.Winner != nil {
		w := *m.Winner
		c.Winner = &w
	}
	return c
}
