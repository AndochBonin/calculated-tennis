package models

import "time"

// CareerMatches is the full merged career match list for a player (classic + Career.js).
type CareerMatches struct {
	PlayerSlug string
	Matches    []RecentResult
	FetchedAt  time.Time
}

// PlayerStats holds parsed Tennis Abstract player page data.
type PlayerStats struct {
	PlayerSlug         string
	RecentResults      []RecentResult
	TourLevelSeasons   []TourLevelSeason
	ChallengerSeasons  []TourLevelSeason // same columns as tour-level; optional on TA pages
	FetchedAt          time.Time         // set by client on successful fetch
}

// RecentResult is one row from the Recent Results table.
type RecentResult struct {
	Date           time.Time // e.g. 06-May-2026
	Tournament     string
	TournamentURL  string
	Surface        string
	Round          string
	Rank           int // Rk; 0 if missing
	OpponentRank   int // vRk; 0 if missing
	Score          string
	DominanceRatio *float64 // DR; nil when the page shows "-"
	AcePct         float64
	DFPct          float64
	FirstServeIn   float64
	FirstServeWon  float64
	SecondServeWon float64
	BPSaved        string // e.g. "6/10"
	Duration       string // Time column
}

// TourLevelSeason is one row from the Tour-Level Seasons table.
type TourLevelSeason struct {
	Year            int
	IsCareer        bool // true when the year cell is "Career"
	Matches         int
	Wins            int
	Losses          int
	WinPct          float64
	SetWL           string // e.g. "55-25"
	SetPct          float64
	GameWL          string
	GamePct         float64
	TiebreakWL      string
	TiebreakPct     float64
	MatchStats      int // MS
	HoldPct         float64
	BreakPct        float64
	AcePct          float64
	DFPct           float64
	FirstServeIn    float64
	FirstServeWon   float64
	SecondServeWon  float64
	SPW             float64
	RPW             float64
	TPW             float64
	DR              float64
	Best            string
	YearURL         string // optional link on the year cell
}
