package diff

// Update represents a single pending change to be applied to MAL.
// Each Update maps to exactly one PATCH API call in Phase 7.
type Update struct {
	AnimeID  int
	Title    string
	Status   string
	Episodes int
}

// WatchlistEntry represents a single entry from the local watchlist.json.
// HiAnime export does not include episode counts — status only.
type WatchlistEntry struct {
	Link          string `json:"link"`
	Name          string `json:"name"`
	MALID         int    `json:"mal_id"`
	WatchListType int    `json:"watchListType"`
}
