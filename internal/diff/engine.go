package diff

import (
	"fmt"

	"github.com/jyotil-raval/mal-updater/internal/mal"
)

// watchListTypeToStatus maps HiAnime integer codes to MAL status strings
var watchListTypeToStatus = map[int]string{
	1: "watching",
	2: "on_hold",
	3: "plan_to_watch",
	4: "dropped",
	5: "completed",
}

// Compare takes the local watchlist and the live MAL entries,
// and returns a slice of Updates — entries whose status differs from MAL.
// If status is completed and total episodes are known, episodes are set to total.
// Otherwise episode count is always preserved from MAL.
func Compare(local []WatchlistEntry, remote []mal.ListEntry) ([]Update, error) {
	malIndex := make(map[int]mal.ListEntry, len(remote))
	for _, entry := range remote {
		malIndex[entry.Node.ID] = entry
	}

	var updates []Update

	for _, localEntry := range local {
		targetStatus, ok := watchListTypeToStatus[localEntry.WatchListType]
		if !ok {
			fmt.Printf("warning: unknown watchListType %d for %q — skipping\n",
				localEntry.WatchListType, localEntry.Name)
			continue
		}

		malEntry, found := malIndex[localEntry.MALID]
		if !found {
			continue
		}

		if targetStatus == malEntry.ListStatus.Status {
			continue
		}

		// Determine target episode count
		targetEpisodes := malEntry.ListStatus.NumEpisodesWatched // default: preserve MAL value

		if targetStatus == "completed" && malEntry.Node.NumEpisodes > 0 {
			// Total episodes known — mark all as watched
			targetEpisodes = malEntry.Node.NumEpisodes
		}

		updates = append(updates, Update{
			AnimeID:  localEntry.MALID,
			Title:    localEntry.Name,
			Status:   targetStatus,
			Episodes: targetEpisodes,
		})
	}

	return updates, nil
}
