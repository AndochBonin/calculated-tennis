package tennis

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrScoreEmpty         = errors.New("tennis: score is empty")
	ErrScoreInvalidToken  = errors.New("tennis: invalid score token")
	ErrScoreInvalidSet    = errors.New("tennis: invalid set score")
	ErrScoreMatchComplete = errors.New("tennis: score includes sets after match is complete")
	ErrScoreTooManySets   = errors.New("tennis: too many sets for format")
	ErrScoreHydration     = errors.New("tennis: score could not be hydrated to match state")
)

var setTokenRE = regexp.MustCompile(`^(\d+)-(\d+)(?:\((\d+)-(\d+)\))?$`)

// SetScore is one set token from a match score string (e.g. 7-5 or 6-6(3-2)).
// TiebreakA and TiebreakB are set only at the tiebreak threshold (typically 6-6).
// nil with equal games at that threshold means tiebreak 0-0; nil otherwise means
// regular games.
type SetScore struct {
	GamesA, GamesB     int
	TiebreakA, TiebreakB *int
}

// ParseScore splits a space-separated match score into set tokens.
func ParseScore(score string) ([]SetScore, error) {
	score = strings.TrimSpace(score)
	if score == "" {
		return nil, ErrScoreEmpty
	}

	tokens := strings.Fields(score)
	sets := make([]SetScore, 0, len(tokens))
	for _, tok := range tokens {
		m := setTokenRE.FindStringSubmatch(tok)
		if m == nil {
			return nil, fmt.Errorf("%w: %q", ErrScoreInvalidToken, tok)
		}

		ga, _ := strconv.Atoi(m[1])
		gb, _ := strconv.Atoi(m[2])

		s := SetScore{GamesA: ga, GamesB: gb}
		if m[3] != "" {
			if ga != gb {
				return nil, fmt.Errorf("%w: tiebreak points only allowed at %d-%d: %q",
					ErrScoreInvalidSet, ga, gb, tok)
			}
			tba, _ := strconv.Atoi(m[3])
			tbb, _ := strconv.Atoi(m[4])
			s.TiebreakA = &tba
			s.TiebreakB = &tbb
		}

		sets = append(sets, s)
	}
	return sets, nil
}

// ValidateScoreSequence checks that parsed sets form a legal sequence for format.
// All tokens except the last must be completed sets; the last may be completed or
// in progress. Completed sets use plain game totals (7-6, not 6-6(7-5)).
func ValidateScoreSequence(format MatchFormat, sets []SetScore) error {
	if len(sets) == 0 {
		return ErrScoreEmpty
	}

	maxSets := 2*format.SetsToWin - 1
	if len(sets) > maxSets {
		return ErrScoreTooManySets
	}

	setsWon := [2]int{}
	for i, s := range sets {
		isLast := i == len(sets)-1

		if err := validateGameTotals(s, format); err != nil {
			return err
		}

		completed, winner, err := classifySet(s, format, setsWon)
		if err != nil {
			return err
		}

		if !isLast {
			if !completed {
				return fmt.Errorf("%w: in-progress set must be last token", ErrScoreInvalidSet)
			}
			setsWon[winner.index()]++
			if setsWon[0] >= format.SetsToWin || setsWon[1] >= format.SetsToWin {
				return ErrScoreMatchComplete
			}
			continue
		}

		if completed {
			setsWon[winner.index()]++
		} else {
			if setsWon[0] >= format.SetsToWin || setsWon[1] >= format.SetsToWin {
				return ErrScoreMatchComplete
			}
			if err := validateInProgressSet(s, format, setsWon); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateGameTotals(s SetScore, format MatchFormat) error {
	if impossibleGameScore(s.GamesA, s.GamesB, format) {
		return ErrScoreInvalidSet
	}
	if s.TiebreakA != nil {
		if s.GamesA != s.GamesB {
			return fmt.Errorf("%w: tiebreak points only at equal game totals", ErrScoreInvalidSet)
		}
		if s.GamesA != format.TiebreakAtGamesEach {
			return fmt.Errorf("%w: tiebreak points only allowed at %d-%d",
				ErrScoreInvalidSet, format.TiebreakAtGamesEach, format.TiebreakAtGamesEach)
		}
	}
	return nil
}

func impossibleGameScore(gamesA, gamesB int, format MatchFormat) bool {
	if gamesA == gamesB {
		return false
	}
	hi, lo := gamesA, gamesB
	if gamesB > gamesA {
		hi, lo = gamesB, gamesA
	}
	maxGames := format.TiebreakAtGamesEach + 1
	if hi > maxGames {
		return true
	}
	if hi == maxGames {
		// Valid completed lines: 7-6 (TB) and 7-5 (regular extended set).
		return lo != format.TiebreakAtGamesEach && lo != format.TiebreakAtGamesEach-1
	}
	return false
}

func classifySet(s SetScore, format MatchFormat, setsWon [2]int) (completed bool, winner Player, err error) {
	ga, gb := s.GamesA, s.GamesB

	if ga == format.TiebreakAtGamesEach && gb == format.TiebreakAtGamesEach {
		tbA, tbB := tiebreakPoints(s)
		if tiebreakComplete(tbA, tbB, format, setsWon) {
			return false, 0, fmt.Errorf("%w: completed tiebreak set must use %d-%d game line",
				ErrScoreInvalidSet, format.TiebreakAtGamesEach+1, format.TiebreakAtGamesEach)
		}
		return false, 0, nil
	}

	if ga == format.TiebreakAtGamesEach+1 && gb == format.TiebreakAtGamesEach {
		if s.TiebreakA != nil {
			return false, 0, fmt.Errorf("%w: completed set must not include tiebreak points", ErrScoreInvalidSet)
		}
		return true, A, nil
	}
	if gb == format.TiebreakAtGamesEach+1 && ga == format.TiebreakAtGamesEach {
		if s.TiebreakA != nil {
			return false, 0, fmt.Errorf("%w: completed set must not include tiebreak points", ErrScoreInvalidSet)
		}
		return true, B, nil
	}

	hi, lo := ga, gb
	if gb > ga {
		hi, lo = gb, ga
	}
	if hi >= format.GamesPerSet && hi-lo >= format.GameMargin {
		if s.TiebreakA != nil {
			return false, 0, fmt.Errorf("%w: tiebreak points only allowed at %d-%d",
				ErrScoreInvalidSet, format.TiebreakAtGamesEach, format.TiebreakAtGamesEach)
		}
		w := A
		if gb > ga {
			w = B
		}
		return true, w, nil
	}

	if s.TiebreakA != nil {
		return false, 0, fmt.Errorf("%w: tiebreak points only allowed at %d-%d",
			ErrScoreInvalidSet, format.TiebreakAtGamesEach, format.TiebreakAtGamesEach)
	}

	return false, 0, nil
}

func validateInProgressSet(s SetScore, format MatchFormat, setsWon [2]int) error {
	ga, gb := s.GamesA, s.GamesB

	if ga == format.TiebreakAtGamesEach && gb == format.TiebreakAtGamesEach {
		tbA, tbB := tiebreakPoints(s)
		if tiebreakComplete(tbA, tbB, format, setsWon) {
			return fmt.Errorf("%w: completed tiebreak set must use %d-%d game line",
				ErrScoreInvalidSet, format.TiebreakAtGamesEach+1, format.TiebreakAtGamesEach)
		}
		return nil
	}

	if s.TiebreakA != nil {
		return fmt.Errorf("%w: tiebreak points only allowed at %d-%d",
			ErrScoreInvalidSet, format.TiebreakAtGamesEach, format.TiebreakAtGamesEach)
	}

	// Scores that would already be a completed set cannot be in progress.
	if completed, _, err := classifySet(s, format, setsWon); err != nil {
		return err
	} else if completed {
		return fmt.Errorf("%w: set %d-%d is already complete",
			ErrScoreInvalidSet, ga, gb)
	}

	return nil
}

func tiebreakPoints(s SetScore) (int, int) {
	if s.TiebreakA != nil {
		return *s.TiebreakA, *s.TiebreakB
	}
	return 0, 0
}

func tiebreakComplete(tbA, tbB int, format MatchFormat, setsWon [2]int) bool {
	ptsToWin := format.TiebreakPointsToWin
	if format.FinalSetTiebreakPointsToWin > 0 {
		need := format.SetsToWin - 1
		if setsWon[0] == need && setsWon[1] == need {
			ptsToWin = format.FinalSetTiebreakPointsToWin
		}
	}

	for _, pts := range []int{tbA, tbB} {
		other := tbA + tbB - pts
		if pts >= ptsToWin && pts-other >= format.TiebreakPointMargin {
			return true
		}
	}
	return false
}

// MatchFromScore builds match state by replaying games with a fixed canonical order.
// Games and tiebreak points are awarded to the player with fewer in the current set;
// on a tie, player A is awarded (see canonicalGameWinner).
func MatchFromScore(format MatchFormat, firstServer Player, score string) (*Match, error) {
	sets, err := ParseScore(score)
	if err != nil {
		return nil, err
	}
	if err := validateScoreSequenceFn(format, sets); err != nil {
		return nil, err
	}

	m := NewMatch(firstServer, format)
	setsWon := [2]int{}

	for i, s := range sets {
		completed, _, err := classifySet(s, format, setsWon)
		if err != nil {
			return nil, err
		}

		if err := hydrateSetFn(m, s, format, completed); err != nil {
			return nil, err
		}

		if completed {
			if len(m.CompletedSets) == 0 {
				return nil, ErrScoreHydration
			}
			last := m.CompletedSets[len(m.CompletedSets)-1]
			if last.GamesA != s.GamesA || last.GamesB != s.GamesB {
				return nil, ErrScoreHydration
			}
			w := A
			if last.GamesB > last.GamesA {
				w = B
			}
			setsWon[w.index()]++
		} else if i == len(sets)-1 {
			if err := assertInProgressSet(m, s, format); err != nil {
				return nil, err
			}
		}
	}

	if m.Done {
		return nil, ErrScoreMatchComplete
	}
	return m, nil
}

// canonicalGameWinner picks who receives the next game or tiebreak point during
// hydration when both sides still need points: the player with fewer in the tally;
// on a tie, A.
func canonicalGameWinner(a, b int) Player {
	if a < b {
		return A
	}
	if b < a {
		return B
	}
	return A
}

// pickNeededGameWinner awards the next game toward targetGA-targetGB. If only one
// player is below their target game count, they receive the game; otherwise the
// canonical fewer-games rule applies among players who still need games.
func pickNeededGameWinner(ga, gb, targetGA, targetGB int) Player {
	needA := ga < targetGA
	needB := gb < targetGB
	if needA && !needB {
		return A
	}
	if needB && !needA {
		return B
	}
	return canonicalGameWinner(ga, gb)
}

// pickNeededTiebreakWinner awards the next tiebreak point toward target totals.
func pickNeededTiebreakWinner(ta, tb, targetA, targetB int) Player {
	needA := ta < targetA
	needB := tb < targetB
	if needA && !needB {
		return A
	}
	if needB && !needA {
		return B
	}
	return canonicalGameWinner(ta, tb)
}

func scoreNeedsTiebreakHydration(s SetScore, format MatchFormat, complete bool) bool {
	tb := format.TiebreakAtGamesEach
	if s.GamesA == tb+1 && s.GamesB == tb {
		return true
	}
	if s.GamesB == tb+1 && s.GamesA == tb {
		return true
	}
	if s.GamesA == tb && s.GamesB == tb {
		return s.TiebreakA != nil || !complete
	}
	return false
}

// validateScoreSequenceFn is used by MatchFromScore; tests may replace it.
var validateScoreSequenceFn = ValidateScoreSequence

// hydrateSetFn is the implementation used by MatchFromScore; tests may replace it.
var hydrateSetFn = hydrateSet

func hydrateSet(m *Match, s SetScore, format MatchFormat, complete bool) error {
	if m.Done {
		return ErrMatchComplete
	}

	if scoreNeedsTiebreakHydration(s, format, complete) {
		tb := format.TiebreakAtGamesEach
		if err := replayRegularGamesTo(m, tb, tb); err != nil {
			return err
		}
		if complete {
			return replayTiebreakUntilSetComplete(m, s.GamesA, s.GamesB)
		}
		tba, tbb := tiebreakPoints(s)
		return replayTiebreakPointsTo(m, tba, tbb)
	}

	if complete {
		return replayRegularGamesUntilSetComplete(m, s.GamesA, s.GamesB)
	}
	return replayRegularGamesTo(m, s.GamesA, s.GamesB)
}

func replayRegularGamesTo(m *Match, targetGA, targetGB int) error {
	for {
		if m.Phase() == Tiebreak {
			ga, gb := m.CurrentSetGames()
			if ga == targetGA && gb == targetGB {
				return nil
			}
			return ErrScoreHydration
		}
		ga, gb := m.CurrentSetGames()
		if ga == targetGA && gb == targetGB {
			return nil
		}
		if ga > targetGA || gb > targetGB {
			return ErrScoreHydration
		}
		w := pickNeededGameWinner(ga, gb, targetGA, targetGB)
		if err := m.WinGame(w); err != nil {
			return err
		}
	}
}

func replayRegularGamesUntilSetComplete(m *Match, targetGA, targetGB int) error {
	before := len(m.CompletedSets)
	for len(m.CompletedSets) == before {
		if m.Phase() == Tiebreak {
			return ErrScoreHydration
		}
		ga, gb := m.CurrentSetGames()
		w := pickNeededGameWinner(ga, gb, targetGA, targetGB)
		if err := m.WinGame(w); err != nil {
			return err
		}
	}
	last := m.CompletedSets[len(m.CompletedSets)-1]
	if last.GamesA != targetGA || last.GamesB != targetGB {
		return ErrScoreHydration
	}
	return nil
}

func replayTiebreakPointsTo(m *Match, targetA, targetB int) error {
	for {
		if m.Phase() != Tiebreak {
			return ErrScoreHydration
		}
		ta, tb := m.TiebreakPoints()
		if ta == targetA && tb == targetB {
			return nil
		}
		if ta > targetA || tb > targetB {
			return ErrScoreHydration
		}
		before := len(m.CompletedSets)
		w := pickNeededTiebreakWinner(ta, tb, targetA, targetB)
		if err := m.WinTiebreakPoint(w); err != nil {
			return err
		}
		if len(m.CompletedSets) > before {
			return ErrScoreHydration
		}
	}
}

func tiebreakClincher(m *Match, targetGA, targetGB int) (Player, bool) {
	ta, tb := m.TiebreakPoints()
	ptsToWin := m.tiebreakPointsToWin()
	margin := m.Format.TiebreakPointMargin
	tbGames := m.Format.TiebreakAtGamesEach

	for _, p := range []Player{A, B} {
		nta, ntbb := ta, tb
		if p == A {
			nta++
		} else {
			ntbb++
		}
		winnerPts := nta
		loserPts := ntbb
		if p == B {
			winnerPts, loserPts = ntbb, nta
		}
		if winnerPts < ptsToWin || winnerPts-loserPts < margin {
			continue
		}
		nga, ngb := tbGames+1, tbGames
		if p == B {
			nga, ngb = tbGames, tbGames+1
		}
		if nga == targetGA && ngb == targetGB {
			return p, true
		}
	}
	return 0, false
}

func pickTiebreakProgressWinner(m *Match, targetGA, targetGB int) Player {
	if w, ok := tiebreakClincher(m, targetGA, targetGB); ok {
		return w
	}
	ta, tb := m.TiebreakPoints()
	setWinner := A
	if targetGB > targetGA {
		setWinner = B
	}
	if ta == tb {
		return setWinner
	}
	return canonicalGameWinner(ta, tb)
}

func replayTiebreakUntilSetComplete(m *Match, targetGA, targetGB int) error {
	before := len(m.CompletedSets)
	for len(m.CompletedSets) == before {
		if m.Phase() != Tiebreak {
			return ErrScoreHydration
		}
		w := pickTiebreakProgressWinner(m, targetGA, targetGB)
		if err := m.WinTiebreakPoint(w); err != nil {
			return err
		}
	}
	last := m.CompletedSets[len(m.CompletedSets)-1]
	if last.GamesA != targetGA || last.GamesB != targetGB {
		return ErrScoreHydration
	}
	return nil
}

func assertInProgressSet(m *Match, s SetScore, format MatchFormat) error {
	ga, gb := m.CurrentSetGames()
	if ga != s.GamesA || gb != s.GamesB {
		return fmt.Errorf("%w: games %d-%d, want %d-%d",
			ErrScoreHydration, ga, gb, s.GamesA, s.GamesB)
	}

	tb := format.TiebreakAtGamesEach
	if s.GamesA == tb && s.GamesB == tb {
		if m.Phase() != Tiebreak {
			return fmt.Errorf("%w: phase %v, want Tiebreak at %d-%d",
				ErrScoreHydration, m.Phase(), tb, tb)
		}
		tba, tbb := tiebreakPoints(s)
		ta, gotB := m.TiebreakPoints()
		if ta != tba || gotB != tbb {
			return fmt.Errorf("%w: tiebreak %d-%d, want %d-%d",
				ErrScoreHydration, ta, gotB, tba, tbb)
		}
		return nil
	}

	if s.TiebreakA != nil {
		return fmt.Errorf("%w: tiebreak points only allowed at %d-%d",
			ErrScoreInvalidSet, tb, tb)
	}
	if m.Phase() != Regular {
		return fmt.Errorf("%w: phase %v, want Regular", ErrScoreHydration, m.Phase())
	}
	return nil
}
