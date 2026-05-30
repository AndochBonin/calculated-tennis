package models

import "time"

// SportsEvent is the production sports WebSocket payload (gameId-centric wire).
type SportsEvent struct {
	GameID             int64             `json:"gameId"`
	LeagueAbbreviation string            `json:"leagueAbbreviation"`
	HomeTeam           string            `json:"homeTeam"`
	AwayTeam           string            `json:"awayTeam"`
	Status             string            `json:"status"`
	Score              string            `json:"score"`
	Period             string            `json:"period"`
	Live               bool              `json:"live"`
	Ended              bool              `json:"ended"`
	EventState         SportsEventState  `json:"eventState"`
}

// SportsEventState is nested state for a sports game (shape varies by sport).
type SportsEventState struct {
	Type           string    `json:"type"`
	StartTime      time.Time `json:"startTime"`
	LastUpdate     time.Time `json:"lastUpdate"`
	Score          string    `json:"score"`
	Period         string    `json:"period"`
	Live           bool      `json:"live"`
	Ended          bool      `json:"ended"`
	TournamentName string    `json:"tournamentName"`
	TennisRound    string    `json:"tennisRound"`
}
