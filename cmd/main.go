// cmd/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/jyotil-raval/mal-updater/internal/config"
	"github.com/jyotil-raval/mal-updater/internal/diff"
	"github.com/jyotil-raval/mal-updater/internal/mal"
	"github.com/jyotil-raval/mal-updater/internal/updater"
	"github.com/jyotil-raval/mal-updater/internal/session"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "Print planned updates without applying them")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	if os.Getenv("MAL_CLIENT_ID") == "" {
		log.Fatal("MAL_CLIENT_ID is not set in .env")
	}
	if os.Getenv("MAL_REDIRECT_URI") == "" {
		log.Fatal("MAL_REDIRECT_URI is not set in .env")
	}

	fmt.Println("Environment loaded successfully.")

	tok, err := session.LoadOrRefresh()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFetching anime list from MAL...")
	malEntries, err := mal.GetAnimeList(tok.AccessToken)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Fetched %d entries from MAL\n", len(malEntries))

	fmt.Println("\nLoading local watchlist...")
	watchlist, err := diff.LoadWatchlist("watchlist.json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Loaded %d entries from watchlist\n", len(watchlist))

	fmt.Println("\nComparing watchlist against MAL...")
	updates, err := diff.Compare(watchlist, malEntries)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d updates needed\n", len(updates))

	if len(updates) == 0 {
		fmt.Println("MAL is already in sync. Nothing to do.")
		return
	}

	if *dryRun {
		fmt.Println("\n[DRY RUN] No changes will be applied.\n")
		for _, u := range updates {
			fmt.Printf("  ~ [%d] %s → status: %s, episodes: %d\n",
				u.AnimeID, u.Title, u.Status, u.Episodes)
		}
		fmt.Printf("\n[DRY RUN] %d updates would be applied.\n", len(updates))
		return
	}

	fmt.Printf("\nApplying %d updates (concurrency: %d)...\n",
		len(updates), config.MALUpdateConcurrency)

	errs := updater.ApplyUpdates(updates, tok.AccessToken)

	fmt.Printf("\nDone. %d succeeded, %d failed.\n",
		len(updates)-len(errs), len(errs))

	if len(errs) > 0 {
		fmt.Println("\nFailed updates:")
		for _, e := range errs {
			log.Printf("  %v", e)
		}
		os.Exit(1)
	}
}
