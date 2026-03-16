package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jyotil-raval/mal-updater/internal/diff"
	"github.com/jyotil-raval/mal-updater/internal/mal"
	"github.com/jyotil-raval/mal-updater/internal/updater"
)

type syncRequest struct {
	Watchlist []diff.WatchlistEntry `json:"watchlist"`
	DryRun    bool                  `json:"dry_run"`
}

type syncResponse struct {
	Total     int  `json:"total"`
	Succeeded int  `json:"succeeded"`
	Failed    int  `json:"failed"`
	DryRun    bool `json:"dry_run"`
}

func (h *Handlers) Sync(w http.ResponseWriter, r *http.Request) {
	var req syncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Watchlist) == 0 {
		writeError(w, http.StatusBadRequest, "watchlist is empty")
		return
	}

	malEntries, err := mal.GetAnimeList(h.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch MAL list")
		return
	}

	updates, err := diff.Compare(req.Watchlist, malEntries)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "diff failed")
		return
	}

	if req.DryRun {
		writeJSON(w, http.StatusOK, syncResponse{
			Total:     len(updates),
			Succeeded: 0,
			Failed:    0,
			DryRun:    true,
		})
		return
	}

	errs := updater.ApplyUpdates(updates, h.AccessToken)

	writeJSON(w, http.StatusOK, syncResponse{
		Total:     len(updates),
		Succeeded: len(updates) - len(errs),
		Failed:    len(errs),
		DryRun:    false,
	})
}
