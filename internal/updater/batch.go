package updater

import (
	"fmt"
	"sync"

	"github.com/jyotil-raval/mal-updater/internal/config"
	"github.com/jyotil-raval/mal-updater/internal/diff"
)

// ApplyUpdates sends all updates to MAL concurrently.
// A semaphore caps concurrent requests to config.MALUpdateConcurrency.
// Errors are collected — one failed PATCH does not abort the rest.
func ApplyUpdates(updates []diff.Update, accessToken string) []error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	// Semaphore — buffered channel caps concurrent goroutines
	sem := make(chan struct{}, config.MALUpdateConcurrency)

	for _, update := range updates {
		wg.Add(1)

		// Capture loop variable — critical in goroutines
		u := update

		go func() {
			defer wg.Done()

			// Acquire semaphore slot — blocks if capacity reached
			sem <- struct{}{}
			defer func() { <-sem }() // Release on exit

			err := PatchAnime(u, accessToken)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				fmt.Printf("  ✗ [%d] %s — %v\n", u.AnimeID, u.Title, err)
				return
			}

			fmt.Printf("  ✓ [%d] %s → %s (%d eps)\n",
				u.AnimeID, u.Title, u.Status, u.Episodes)
		}()
	}

	// Block until all goroutines finish
	wg.Wait()

	return errs
}
