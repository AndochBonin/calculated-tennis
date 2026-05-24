package tennis

import "errors"

var (
	ErrMatchComplete = errors.New("tennis: match is complete")
	ErrWrongPhase    = errors.New("tennis: wrong phase for this operation")
)

// WinGame records that winner won the current regular game.
func (m *Match) WinGame(winner Player) error {
	if m.Done {
		return ErrMatchComplete
	}
	if m.Current.Phase != Regular {
		return ErrWrongPhase
	}

	w := winner.index()
	l := winner.Opponent().index()
	m.Current.Games[w]++

	winnerGames := m.Current.Games[w]
	loserGames := m.Current.Games[l]

	if m.Current.Games[0] == m.Format.TiebreakAtGamesEach &&
		m.Current.Games[1] == m.Format.TiebreakAtGamesEach {
		m.server = m.server.Opponent() // switch server for the tiebreak
		m.enterTiebreak()
		return nil // we do not complete set yet - we must wait for the tiebreak to be completed
	}

	if winnerGames >= m.Format.GamesPerSet && winnerGames-loserGames >= m.Format.GameMargin {
		return m.completeSet(winner)
	}

	m.server = m.server.Opponent()
	return nil
}

func (m *Match) enterTiebreak() {
	m.Current.Phase = Tiebreak
	m.Current.TiebreakPoints = [2]int{}
	m.Current.PointsPlayedInTB = 0
	m.Current.FirstServerOfTiebreak = m.server // first server of tiebreak is the player who would have served the next regular game
}

func (m *Match) completeSet(winner Player) error {
	m.CompletedSets = append(m.CompletedSets, SetResult{
		GamesA: m.Current.Games[0],
		GamesB: m.Current.Games[1],
	})

	w := winner.index()
	m.setsWon[w]++

	if m.setsWon[w] >= m.Format.SetsToWin {
		win := winner
		m.Winner = &win
		m.Done = true
		return nil
	}

	m.resetCurrentSet()
	m.server = m.server.Opponent()
	return nil
}

func (m *Match) resetCurrentSet() { // just makes things more readable
	m.Current = Set{}
}

// Phase returns the current set phase.
func (m *Match) Phase() Phase {
	return m.Current.Phase
}

// Server returns who serves the next regular game or tiebreak point.
func (m *Match) Server() Player {
	return m.server
}

// SetsWon returns completed set counts for A and B.
func (m *Match) SetsWon() (a, b int) {
	return m.setsWon[0], m.setsWon[1]
}

// CurrentSetGames returns in-progress game counts for the current set.
func (m *Match) CurrentSetGames() (a, b int) {
	return m.Current.Games[0], m.Current.Games[1]
}

// TiebreakPoints returns tiebreak points in the current set (0 if not in a tiebreak).
func (m *Match) TiebreakPoints() (a, b int) {
	return m.Current.TiebreakPoints[0], m.Current.TiebreakPoints[1]
}

func (m *Match) isDecidingSet() bool {
	need := m.Format.SetsToWin - 1
	return m.setsWon[0] == need && m.setsWon[1] == need
}

func (m *Match) tiebreakPointsToWin() int {
	if m.Format.FinalSetTiebreakPointsToWin > 0 && m.isDecidingSet() {
		return m.Format.FinalSetTiebreakPointsToWin
	}
	return m.Format.TiebreakPointsToWin
}

// WinTiebreakPoint records that winner won the current tiebreak point.
func (m *Match) WinTiebreakPoint(winner Player) error {
	if m.Done {
		return ErrMatchComplete
	}
	if m.Current.Phase != Tiebreak {
		return ErrWrongPhase
	}

	w := winner.index()
	l := winner.Opponent().index()
	m.Current.TiebreakPoints[w]++
	m.Current.PointsPlayedInTB++

	winnerPts := m.Current.TiebreakPoints[w]
	loserPts := m.Current.TiebreakPoints[l]

	if winnerPts >= m.tiebreakPointsToWin() && winnerPts-loserPts >= m.Format.TiebreakPointMargin {
		m.Current.Games[w] = m.Format.TiebreakAtGamesEach + 1
		m.Current.Games[l] = m.Format.TiebreakAtGamesEach
		m.server = m.Current.FirstServerOfTiebreak // this is so the next set's first game server is the one who RECEIVED the first point in the tiebreak
		return m.completeSet(winner) // because we change server in this function
	}

	m.server = serverForTBPoint(m.Current.PointsPlayedInTB, m.Current.FirstServerOfTiebreak)
	return nil
}

// serverForTBPoint returns who serves tiebreak point n+1 (n is count of points already played).
// ITF rotation: point 1 first server; 2–3 opponent; 4–5 first; so on and so forth 
// (alternate every 2 points after first serve)
func serverForTBPoint(n int, first Player) Player {
	if n == 0 {
		return first
	}
	block := (n - 1) / 2
	if block%2 == 0 { // if block is even, the server is the first player's opponent
		return first.Opponent()
	}
	return first
}