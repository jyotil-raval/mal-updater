package mal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jyotil-raval/mal-updater/internal/config"
)

// GetAnimeList fetches the authenticated user's full anime list from MAL.
// It handles pagination automatically and returns all entries as a slice.
func GetAnimeList(accessToken string) ([]ListEntry, error) {
	var results []ListEntry
	offset := 0

	for {
		page, err := fetchPage(accessToken, offset)
		if err != nil {
			return nil, fmt.Errorf("fetching page at offset %d: %w", offset, err)
		}

		results = append(results, page.Data...)

		if page.Paging.Next == "" {
			break
		}

		offset += config.MALListLimit
	}

	return results, nil
}

// fetchPage makes a single GET request to the MAL animelist endpoint
// at the given offset and returns the decoded page response
func fetchPage(accessToken string, offset int) (*pageResponse, error) {
	params := url.Values{}
	params.Set("fields", "list_status,num_episodes,media_type")
	params.Set("limit", strconv.Itoa(config.MALListLimit))
	params.Set("offset", strconv.Itoa(offset))

	endpoint := config.MALAPIBaseURL + "/users/@me/animelist?" + params.Encode()

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MAL API returned status %d", resp.StatusCode)
	}

	var page pageResponse
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &page, nil
}

// pageResponse is the raw shape of a single paginated MAL API response
type pageResponse struct {
	Data   []ListEntry `json:"data"`
	Paging struct {
		Next string `json:"next"`
	} `json:"paging"`
}
