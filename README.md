# mal-updater

Automate MyAnimeList updates from a local watchlist file — built in Go.

---

## What It Does

`mal-updater` is a CLI tool and HTTP API that reads your anime watchlist from a local
`watchlist.json` file, compares it against your live MyAnimeList account,
and PATCHes only the entries that differ — concurrently.

No manual MAL updates. No full list replacements. Just the delta.

**Two modes:**

- **CLI** — run locally, sync on demand
- **HTTP Server** — expose a JWT-protected REST API, consumable as a microservice

---

## How It Works

1. Authenticates with MAL via OAuth2 + PKCE (no client secret required)
2. Reads `watchlist.json` — your desired list state
3. Fetches your current MAL list via the MAL v2 API
4. Diffs the two states
5. Applies only the changed entries, concurrently

---

## Watchlist Format

Two formats are supported — auto-detected at runtime.

### Format A — Categorised Object (HiAnime export)

```json
{
  "Watching": [{ "link": "https://myanimelist.net/anime/21", "name": "One Piece", "mal_id": 21, "watchListType": 1 }],
  "Completed": [{ "link": "https://myanimelist.net/anime/1535", "name": "Death Note", "mal_id": 1535, "watchListType": 5 }],
  "Plan to Watch": [],
  "On-Hold": [],
  "Dropped": []
}
```

### Format B — Flat Array

```json
[
  { "link": "https://myanimelist.net/anime/21", "name": "One Piece", "mal_id": 21, "watchListType": 1 },
  { "link": "https://myanimelist.net/anime/1535", "name": "Death Note", "mal_id": 1535, "watchListType": 5 }
]
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

- Go 1.26 or higher
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
cp .env.example.env
```

Open `.env` and fill in your credentials:

```env
MAL_CLIENT_ID=<your_client_id_here>
MAL_REDIRECT_URI=http://localhost:8080/callback
JWT_SECRET=your_secret_key_here
SERVER_PORT=8080
```

### Add Your Watchlist

Export your watchlist from HiAnime and save it as `watchlist.json` in the project root.
Use `watchlist.example.json` as a format reference.

---

## Usage

### CLI

```bash
# Full sync
go run cmd/main.go

# Preview without applying
go run cmd/main.go --dry-run
```

On first run, a browser window opens for MAL authentication.
Subsequent runs use the cached token silently.

### HTTP Server _(coming in Phase 11)_

```bash
go run cmd/server/main.go
```

```bash
# Issue a JWT token
curl -X POST http://localhost:8080/auth/token

# Sync via API
curl -X POST http://localhost:8080/sync \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"watchlist": [{"mal_id": 1535, "watchListType": 5}]}'

# Update single entry
curl -X PATCH http://localhost:8080/anime/1535 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"status": "completed"}'

# Get anime details
curl http://localhost:8080/anime/1535 \
  -H "Authorization: Bearer <token>"

# Search anime
curl "http://localhost:8080/anime/search?q=naruto&genre=action&status=finished" \
  -H "Authorization: Bearer <token>"

# Filter your list
curl "http://localhost:8080/list?status=watching&sort=title" \
  -H "Authorization: Bearer <token>"
```

### Docker _(coming in Phase 12)_

```bash
docker compose up
```

---

## API Reference _(coming in Phase 11)_

| Method  | Endpoint        | Auth | Description                                                |
| ------- | --------------- | ---- | ---------------------------------------------------------- |
| `POST`  | `/auth/token`   | None | Issue a JWT token                                          |
| `POST`  | `/sync`         | JWT  | Full watchlist diff + apply                                |
| `PATCH` | `/anime/:id`    | JWT  | Update single MAL entry — auto-fills episodes on completed |
| `GET`   | `/anime/:id`    | JWT  | Full anime details from MAL                                |
| `GET`   | `/anime/search` | JWT  | Search MAL + filter local list                             |
| `GET`   | `/list`         | JWT  | User's MAL list with filters                               |

---

## Project Structure

```
mal-updater/
├── cmd/
│   ├── main.go              ← CLI entry point
│   └── server/
│       └── main.go          ← HTTP server entry point (Phase 11)
├── auth/                    ← OAuth2 + PKCE — public package
│   ├── pkce.go
│   ├── callback.go
│   ├── browser.go
│   ├── exchange.go
│   └── refresh.go
├── token/                   ← Token struct, Save, Load, IsExpired
│   └── token.go
├── internal/
│   ├── config/              ← Constants — endpoints, ports, concurrency caps
│   ├── session/             ← Token lifecycle orchestration — LoadOrRefresh()
│   ├── diff/                ← Watchlist loader + diff engine
│   ├── mal/                 ← MAL v2 API client + pagination
│   ├── updater/             ← Concurrent batch PATCH
│   └── server/              ← HTTP router, middleware, handlers (Phase 11)
├── docs/                    ← Technical documentation
├── watchlist.json           ← Your watchlist (gitignored)
├── watchlist.example.json   ← Format reference
├── .env                     ← Credentials (gitignored)
├── .env.example             ← Credential template
├── Dockerfile               ← Two-stage build (Phase 12)
├── docker-compose.yml       ← Mount + port config (Phase 12)
├── go.mod
└── go.sum
```

---

## Tech

- Go standard library — `net/http`, `crypto/rand`, `encoding/json`, `sync`, `flag`
- [`godotenv`](https://github.com/joho/godotenv) — `.env` file loading
- [`golang-jwt/jwt`](https://github.com/golang-jwt/jwt) — JWT auth (Phase 11)

---

## Phase Progress

| Phase | Description                                         | Status |
| ----- | --------------------------------------------------- | ------ |
| 1     | Environment setup + PKCE generation                 | ✅     |
| 2     | OAuth2 callback server                              | ✅     |
| 3     | Token exchange + storage                            | ✅     |
| 4     | MAL API client + pagination                         | ✅     |
| 5     | Watchlist loader (multi-format)                     | ✅     |
| 6     | Diff engine                                         | ✅     |
| 7     | Concurrent batch updater                            | ✅     |
| 8     | Silent token refresh                                | ✅     |
| 9     | `--dry-run` CLI flag                                | ✅     |
| 10    | Structural refactor — `auth/`, `token/`, `session/` | ✅     |
| 11    | HTTP server + JWT middleware + handlers             | 🔜     |
| 12    | Docker — two-stage build + compose                  | 🔜     |

---

## License

[MIT](./LICENSE)
