package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jyotil-raval/mal-updater/internal/config"
	"github.com/jyotil-raval/mal-updater/internal/diff"
	"github.com/jyotil-raval/mal-updater/internal/mal"
	"github.com/jyotil-raval/mal-updater/internal/updater"
)

type updateRequest struct {
	Status   string `json:"status"`
	Episodes int    `json:"episodes"`
}

type updateResponse struct {
	ID          int  `json:"id"`
	Updated     bool `json:"updated"`
	EpisodesSet int  `json:"episodes_set"`
}

func (h *Handlers) UpdateAnime(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid anime id")
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}

	episodes := req.Episodes

	// Auto-fill episodes when marking completed
	if strings.EqualFold(req.Status, "completed") && episodes == 0 {
		episodes, err = fetchTotalEpisodes(r.Context(), id, h.AccessToken)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch anime details for episode count")
			return
		}
	}

	update := diff.Update{
		AnimeID:  id,
		Status:   req.Status,
		Episodes: episodes,
	}

	errs := updater.ApplyUpdates([]diff.Update{update}, h.AccessToken)
	if len(errs) > 0 {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("update failed: %v", errs[0]))
		return
	}

	writeJSON(w, http.StatusOK, updateResponse{
		ID:          id,
		Updated:     true,
		EpisodesSet: episodes,
	})
}

// fetchTotalEpisodes calls MAL to get the total episode count for an anime
func fetchTotalEpisodes(ctx interface{ Done() <-chan struct{} }, id int, accessToken string) (int, error) {
	endpoint := fmt.Sprintf("%s/anime/%d?fields=num_episodes", config.MALAPIBaseURL, id)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		NumEpisodes int `json:"num_episodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	entry, err := mal.GetAnimeList(accessToken)
	_ = entry
	_ = err

	return result.NumEpisodes, nil
}
