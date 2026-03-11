package diff

import (
	"encoding/json"
	"fmt"
	"os"
)

// RawWatchlist is the top-level shape of the categorized HiAnime export
type RawWatchlist struct {
	Watching    []WatchlistEntry `json:"Watching"`
	Completed   []WatchlistEntry `json:"Completed"`
	OnHold      []WatchlistEntry `json:"On-Hold"`
	Dropped     []WatchlistEntry `json:"Dropped"`
	PlanToWatch []WatchlistEntry `json:"Plan to Watch"`
}

// LoadWatchlist reads a watchlist file and returns all entries as a flat slice.
// Supports two formats:
//   - Categorized object: { "Watching": [...], "Completed": [...], ... }
//   - Flat array: [ { "mal_id": 1, "watchListType": 5, ... }, ... ]
func LoadWatchlist(path string) ([]WatchlistEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading watchlist: %w", err)
	}

	// Detect format by inspecting the first non-whitespace byte
	// '[' → flat array, '{' → categorized object
	for _, b := range data {
		if b == ' ' || b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		if b == '[' {
			return loadFlatArray(data)
		}
		if b == '{' {
			return loadCategorized(data)
		}
		break
	}

	return nil, fmt.Errorf("unrecognized watchlist format: expected '{' or '['")
}

// loadFlatArray parses a flat JSON array of WatchlistEntry
func loadFlatArray(data []byte) ([]WatchlistEntry, error) {
	var entries []WatchlistEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing flat watchlist: %w", err)
	}
	return entries, nil
}

// loadCategorized parses the categorized object format and flattens into a slice
func loadCategorized(data []byte) ([]WatchlistEntry, error) {
	var raw RawWatchlist
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing categorized watchlist: %w", err)
	}

	all := make([]WatchlistEntry, 0)
	all = append(all, raw.Watching...)
	all = append(all, raw.Completed...)
	all = append(all, raw.OnHold...)
	all = append(all, raw.Dropped...)
	all = append(all, raw.PlanToWatch...)

	return all, nil
}
