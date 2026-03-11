# mal-updater

Automate MyAnimeList updates from a local watchlist file — built in Go.

---

## What It Does

`mal-updater` is a CLI tool that reads your anime watchlist from a local
`watchlist.json` file, compares it against your live MyAnimeList account,
and PATCHes only the entries that differ — concurrently.

No manual MAL updates. No full list replacements. Just the delta.

---

## How It Works

1. Authenticates with MAL via OAuth2 + PKCE (no client secret required)
2. Reads `watchlist.json` — your desired list state
3. Fetches your current MAL list via the MAL v2 API
4. Diffs the two states
5. Applies only the changed entries, concurrently

---

## Watchlist Format

The tool reads a `watchlist.json` file exported from [HiAnime](https://hianime.to)
or any compatible source. The format is a categorized object grouped by watch status.

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
  "Plan to Watch": [...],
  "On-Hold": [...],
  "Dropped": [...]
}
```

### `watchListType` Reference

| Value | Status        | MAL Equivalent  |
| ----- | ------------- | --------------- |
| 1     | Watching      | `watching`      |
| 2     | On-Hold       | `on_hold`       |
| 3     | Plan to Watch | `plan_to_watch` |
| 4     | Dropped       | `dropped`       |
| 5     | Completed     | `completed`     |

> See `watchlist.example.json` for a minimal working example.

---

## Setup

### Prerequisites

- Go 1.22 or higher
- A MAL API client ID — register at [myanimelist.net/apiconfig](https://myanimelist.net/apiconfig)
  - App type: `other`
  - Redirect URI: `http://localhost:8080/callback`

### Install

```bash
git clone https://github.com/jyotil-raval/mal-updater.git
cd mal-updater
go mod tidy
```

### Configure

```bash
cp .env.example .env
```

Open `.env` and fill in your credentials:

```env
MAL_CLIENT_ID=your_client_id_here
MAL_REDIRECT_URI=http://localhost:8080/callback
```

### Add Your Watchlist

Export your watchlist from HiAnime and save it as `watchlist.json` in the
project root. Use `watchlist.example.1.json`, `watchlist.example.2.json` as a reference for the expected format.

### Run

```bash
go run cmd/main.go
```

On first run, the tool will open a browser window for MAL authentication.
After approval, it runs silently on subsequent executions using a cached token.

---

## Project Structure

```
mal-updater/
├── cmd/
│   └── main.go              ← entry point
├── internal/
│   ├── auth/                ← OAuth2 + PKCE flow
│   ├── mal/                 ← MAL v2 API client
│   ├── store/               ← token persistence
│   └── diff/                ← watchlist diff engine
├── watchlist.json           ← your watchlist (gitignored)
├── watchlist.example.json   ← format reference
├── .env                     ← credentials (gitignored)
├── .env.example             ← credential template
├── go.mod
└── go.sum
```

---

## Tech

- Go standard library — `net/http`, `crypto/rand`, `encoding/json`, `sync`
- [`godotenv`](https://github.com/joho/godotenv) — `.env` file loading

---

## License

[MIT](./LICENSE)
