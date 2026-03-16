package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jyotil-raval/mal-updater/internal/config"
)

func (h *Handlers) GetAnime(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing anime id")
		return
	}

	fields := strings.Join([]string{
		"id", "title", "synopsis", "media_type", "status",
		"num_episodes", "start_date", "end_date", "mean", "rank",
		"popularity", "rating", "genres", "themes", "demographics", "studios",
	}, ",")

	endpoint := fmt.Sprintf("%s/anime/%s?fields=%s", config.MALAPIBaseURL, id, fields)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build request")
		return
	}
	req.Header.Set("Authorization", "Bearer "+h.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach MAL")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		writeError(w, http.StatusNotFound, "anime not found")
		return
	}
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "MAL returned an error")
		return
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decode MAL response")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) SearchAnime(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	genre := r.URL.Query().Get("genre")
	status := r.URL.Query().Get("status")
	mediaType := r.URL.Query().Get("type")

	if q == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	params := url.Values{}
	params.Set("q", q)
	params.Set("limit", "20")
	params.Set("fields", "id,title,media_type,status,mean,genres,num_episodes")

	if mediaType != "" {
		params.Set("media_type", mediaType)
	}

	endpoint := fmt.Sprintf("%s/anime?%s", config.MALAPIBaseURL, params.Encode())

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build request")
		return
	}
	req.Header.Set("Authorization", "Bearer "+h.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach MAL")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read MAL response")
		return
	}

	var malResult struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &malResult); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decode MAL response")
		return
	}

	// Filter by genre client-side — MAL search API doesn't support genre filtering directly
	results := malResult.Data
	if genre != "" {
		filtered := make([]map[string]any, 0)
		for _, item := range results {
			node, ok := item["node"].(map[string]any)
			if !ok {
				continue
			}
			genres, ok := node["genres"].([]any)
			if !ok {
				continue
			}
			for _, g := range genres {
				gMap, ok := g.(map[string]any)
				if !ok {
					continue
				}
				if name, ok := gMap["name"].(string); ok &&
					strings.EqualFold(name, genre) {
					filtered = append(filtered, item)
					break
				}
			}
		}
		results = filtered
	}

	// Filter by status client-side
	if status != "" {
		filtered := make([]map[string]any, 0)
		for _, item := range results {
			node, ok := item["node"].(map[string]any)
			if !ok {
				continue
			}
			if s, ok := node["status"].(string); ok &&
				strings.EqualFold(s, status) {
				filtered = append(filtered, item)
			}
		}
		results = filtered
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": results})
}
