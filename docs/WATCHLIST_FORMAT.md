# Watchlist Format

> Internal reference for the `mal-updater` project.
> Documents the `watchlist.json` input format, field definitions,
> and how the data maps to MAL API values.

---

## Overview

`watchlist.json` is the source of truth for what your MAL list should look like.
The tool reads this file, compares it against your live MAL list, and applies
only the entries that differ.

The format is a **categorized object** exported from [HiAnime](https://hianime.to),
grouped by watch status. Each key is a status category containing an array of
anime entries.

---

## Full Format

```json
{
  "Watching": [
    {
      "link": "https://myanimelist.net/anime/21",
      "name": "One Piece",
      "mal_id": 21,
      "watchListType": 1
    }
  ],
  "Completed": [
    {
      "link": "https://myanimelist.net/anime/1535",
      "name": "Death Note",
      "mal_id": 1535,
      "watchListType": 5
    }
  ],
  "Plan to Watch": [
    {
      "link": "https://myanimelist.net/anime/5114",
      "name": "Fullmetal Alchemist: Brotherhood",
      "mal_id": 5114,
      "watchListType": 3
    }
  ],
  "On-Hold": [
    {
      "link": "https://myanimelist.net/anime/269",
      "name": "Bleach",
      "mal_id": 269,
      "watchListType": 2
    }
  ],
  "Dropped": [
    {
      "link": "https://myanimelist.net/anime/42544",
      "name": "Fena: Pirate Princess",
      "mal_id": 42544,
      "watchListType": 4
    }
  ]
}
```

---

## Field Definitions

| Field           | Type   | Required | Description                                        |
| --------------- | ------ | -------- | -------------------------------------------------- |
| `mal_id`        | int    | ✅ Yes   | MAL anime ID. Used to identify the entry on MAL.   |
| `name`          | string | ✅ Yes   | Anime title. Used for logging and error messages.  |
| `link`          | string | No       | MAL URL for the anime. Not used by the tool logic. |
| `watchListType` | int    | ✅ Yes   | Numeric status code. Mapped to MAL status strings. |

---

## `watchListType` Reference

The `watchListType` value from HiAnime maps to MAL API status strings as follows:

| `watchListType` | HiAnime Category | MAL API Status  |
| --------------- | ---------------- | --------------- |
| 1               | Watching         | `watching`      |
| 2               | On-Hold          | `on_hold`       |
| 3               | Plan to Watch    | `plan_to_watch` |
| 4               | Dropped          | `dropped`       |
| 5               | Completed        | `completed`     |

This mapping is handled in `internal/diff/watchlist.go`.

---

## What the Tool Uses vs. What It Ignores

The tool only syncs **status** to MAL. Score and episode count are not
present in the HiAnime export and are not sent in PATCH requests unless
explicitly added.

| Data Point               | Source           | Synced to MAL |
| ------------------------ | ---------------- | ------------- |
| Anime ID (`mal_id`)      | `watchlist.json` | Used as key   |
| Status (`watchListType`) | `watchlist.json` | ✅ Yes        |
| Score                    | Not in export    | ❌ No         |
| Episodes watched         | Not in export    | ❌ No         |
| Title (`name`)           | `watchlist.json` | Logging only  |
| Link                     | `watchlist.json` | ❌ Ignored    |

> If you want to sync score or episode count in the future, you would need
> to extend the `watchlist.json` format manually and update the diff engine.

---

## Category Keys — Exact Spelling Required

The object keys must match exactly. The diff engine reads these keys by name:

| Key               | Notes                              |
| ----------------- | ---------------------------------- |
| `"Watching"`      | Capital W, no variation            |
| `"Completed"`     | Capital C                          |
| `"Plan to Watch"` | Spaces, not underscores or hyphens |
| `"On-Hold"`       | Hyphen, capital O and H            |
| `"Dropped"`       | Capital D                          |

If a category key is missing, the tool skips it without error.
If a category key is misspelled, its entries will not be processed.

---

## How to Get Your `watchlist.json`

1. Go to [HiAnime](https://hianime.to) and log in
2. Navigate to your profile → Watchlist
3. Export your list — the downloaded file is your `watchlist.json`
4. Rename it to `watchlist.json` and place it in the project root

> `watchlist.json` is listed in `.gitignore`. It contains your personal
> watch history and should never be committed to the repository.
> Use `watchlist.example.json` as a format reference for contributors.

---

## `watchlist.example.json` Format

A minimal example showing one entry per category:

```json
{
  "Watching": [{ "link": "https://myanimelist.net/anime/21", "name": "One Piece", "mal_id": 21, "watchListType": 1 }],
  "Completed": [{ "link": "https://myanimelist.net/anime/1535", "name": "Death Note", "mal_id": 1535, "watchListType": 5 }],
  "Plan to Watch": [{ "link": "https://myanimelist.net/anime/5114", "name": "Fullmetal Alchemist: Brotherhood", "mal_id": 5114, "watchListType": 3 }],
  "On-Hold": [{ "link": "https://myanimelist.net/anime/269", "name": "Bleach", "mal_id": 269, "watchListType": 2 }],
  "Dropped": [{ "link": "https://myanimelist.net/anime/42544", "name": "Fena: Pirate Princess", "mal_id": 42544, "watchListType": 4 }]
}
```

---

_Last updated: March 2026_
