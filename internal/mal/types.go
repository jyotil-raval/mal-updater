package mal

// ListEntry represents a single anime entry in the user's MAL list
type ListEntry struct {
	Node       Node       `json:"node"`
	ListStatus ListStatus `json:"list_status"`
}

// Node holds the core identity of an anime
type Node struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	NumEpisodes int    `json:"num_episodes"`
	MediaType   string `json:"media_type"` // tv | movie | ova | special | ona | music
}

// ListStatus holds the user's watch state for a given anime
type ListStatus struct {
	Status             string `json:"status"`
	NumEpisodesWatched int    `json:"num_episodes_watched"`
	Score              int    `json:"score"`
}
