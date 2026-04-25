package models

// SportEvent is a live score update received from the CLOB sports WebSocket channel.
type SportEvent struct {
	Slug       string `json:"slug"`
	Live       bool   `json:"live"`
	Ended      bool   `json:"ended"`
	Score      string `json:"score"`
	Period     string `json:"period"`
	Elapsed    string `json:"elapsed"`
	LastUpdate string `json:"last_update"`
}
