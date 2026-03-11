package diff

import (
	"encoding/json"
	"fmt"
	"os"
)

// RawWatchlist is the top-level shape of the HiAnime export
type RawWatchlist struct {
	Watching    []WatchlistEntry `json:"Watching"`
	Completed   []WatchlistEntry `json:"Completed"`
	OnHold      []WatchlistEntry `json:"On-Hold"`
	Dropped     []WatchlistEntry `json:"Dropped"`
	PlanToWatch []WatchlistEntry `json:"Plan to Watch"`
}

// LoadWatchlist reads watchlist.json and returns all entries as a flat slice
func LoadWatchlist(path string) ([]WatchlistEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading watchlist: %w", err)
	}

	var raw RawWatchlist
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing watchlist: %w", err)
	}

	all := make([]WatchlistEntry, 0)
	all = append(all, raw.Watching...)
	all = append(all, raw.Completed...)
	all = append(all, raw.OnHold...)
	all = append(all, raw.Dropped...)
	all = append(all, raw.PlanToWatch...)

	return all, nil
}
