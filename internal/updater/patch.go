package updater

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jyotil-raval/mal-updater/internal/config"
	"github.com/jyotil-raval/mal-updater/internal/diff"
)

// PatchAnime sends a single PATCH request to MAL to update one anime entry.
// It updates both status and episode count in a single call.
func PatchAnime(update diff.Update, accessToken string) error {
	endpoint := fmt.Sprintf("%s/anime/%d/my_list_status",
		config.MALAPIBaseURL, update.AnimeID)

	form := url.Values{}
	form.Set("status", update.Status)
	form.Set("num_watched_episodes", strconv.Itoa(update.Episodes))

	req, err := http.NewRequest("PATCH", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("building PATCH request for %q: %w", update.Title, err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("executing PATCH for %q: %w", update.Title, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PATCH failed for %q (status %d): %s",
			update.Title, resp.StatusCode, string(body))
	}

	return nil
}
