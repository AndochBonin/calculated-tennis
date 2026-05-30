package tennis

// Player identifies side A or B in a match.
type Player int

const (
	A Player = iota
	B
)

// Opponent returns the other player.
func (p Player) Opponent() Player {
	if p == A {
		return B
	}
	return A
}

// index returns 0 for A and 1 for B.
func (p Player) index() int {
	return int(p)
}
