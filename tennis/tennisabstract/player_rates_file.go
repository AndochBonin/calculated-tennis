package tennisabstract

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// PlayerRates2024 is raw season hold/break/DR for one player (calibration cache).
type PlayerRates2024 struct {
	Hold2024  float64 `json:"hold_2024"`
	Break2024 float64 `json:"break_2024"`
	DR2024    float64 `json:"dr_2024,omitempty"`
}

// PlayerRatesMap is slug → 2024 season rates for calibration.
type PlayerRatesMap map[string]PlayerRates2024

// ReadPlayerRatesFile loads a player_rates_2024.json cache.
func ReadPlayerRatesFile(path string) (PlayerRatesMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m PlayerRatesMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decode player rates %q: %w", path, err)
	}
	if m == nil {
		m = PlayerRatesMap{}
	}
	return m, nil
}

// WritePlayerRatesFile writes slug → rates as indented JSON with stable key order.
func WritePlayerRatesFile(path string, rates PlayerRatesMap) error {
	if rates == nil {
		rates = PlayerRatesMap{}
	}
	keys := make([]string, 0, len(rates))
	for slug := range rates {
		keys = append(keys, slug)
	}
	sort.Strings(keys)

	var buf strings.Builder
	buf.WriteString("{\n")
	for i, slug := range keys {
		entry, err := json.Marshal(rates[slug])
		if err != nil {
			return fmt.Errorf("encode rates for %q: %w", slug, err)
		}
		keyJSON, err := json.Marshal(slug)
		if err != nil {
			return fmt.Errorf("encode slug %q: %w", slug, err)
		}
		if i > 0 {
			buf.WriteString(",\n")
		}
		buf.WriteString("  ")
		buf.Write(keyJSON)
		buf.WriteString(": ")
		buf.Write(entry)
	}
	buf.WriteString("\n}\n")

	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("write player rates %q: %w", path, err)
	}
	return nil
}
