package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jyotil-raval/mal-updater/internal/mal"
)

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	mediaType := r.URL.Query().Get("type")
	scoreStr := r.URL.Query().Get("score")
	sort := r.URL.Query().Get("sort")

	entries, err := mal.GetAnimeList(h.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch MAL list")
		return
	}

	// Filter by status
	if status != "" {
		filtered := entries[:0]
		for _, e := range entries {
			if strings.EqualFold(e.ListStatus.Status, status) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Filter by media type
	if mediaType != "" {
		filtered := entries[:0]
		for _, e := range entries {
			if strings.EqualFold(e.Node.MediaType, mediaType) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Filter by minimum score
	if scoreStr != "" {
		minScore, err := strconv.Atoi(scoreStr)
		if err == nil {
			filtered := entries[:0]
			for _, e := range entries {
				if e.ListStatus.Score >= minScore {
					filtered = append(filtered, e)
				}
			}
			entries = filtered
		}
	}

	// Sort
	if sort == "title" {
		for i := 1; i < len(entries); i++ {
			for j := i; j > 0 && entries[j].Node.Title < entries[j-1].Node.Title; j-- {
				entries[j], entries[j-1] = entries[j-1], entries[j]
			}
		}
	} else if sort == "score" {
		for i := 1; i < len(entries); i++ {
			for j := i; j > 0 && entries[j].ListStatus.Score > entries[j-1].ListStatus.Score; j-- {
				entries[j], entries[j-1] = entries[j-1], entries[j]
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total": len(entries),
		"data":  entries,
	})
}
