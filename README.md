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
- **HTTP Server** — JWT-protected REST API, consumable as a microservice

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
cp .env.example .env
```

Open `.env` and fill in your credentials:

```env
MAL_CLIENT_ID=your_client_id_here
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

### HTTP Server

```bash
go run cmd/server/main.go
# → Server running on :8080
```

```bash
# Issue a JWT token
curl -X POST http://localhost:8080/auth/token

# Sync via API
curl -X POST http://localhost:8080/sync \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"watchlist": [{"mal_id": 1535, "watchListType": 5}], "dry_run": false}'

# Update single entry — episodes auto-filled when status is completed
curl -X PATCH http://localhost:8080/anime/1535 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"status": "completed"}'

# Get anime details
curl http://localhost:8080/anime/1535 \
  -H "Authorization: Bearer <token>"

# Search anime
curl "http://localhost:8080/anime/search?q=naruto&genre=action&status=finished_airing" \
  -H "Authorization: Bearer <token>"

# Filter your list
curl "http://localhost:8080/list?status=watching&sort=title" \
  -H "Authorization: Bearer <token>"
```

### Docker

Authenticate locally first to create `token.json` — the container reads it via volume mount:

```bash
go run cmd/main.go   # completes OAuth2 flow, creates token.json
```

Then start the server in Docker:

```bash
docker compose up
# → Server running on :8080
```

Rebuild after code changes:

```bash
docker compose build
docker compose up
```

Stop:

```bash
docker compose down
```

### Bruno (API Collection)

A Bruno collection is included in `bruno/` — open it for a ready-to-use API client with
all endpoints pre-configured and token auto-capture on `Issue Token`.

```bash
# Install Bruno
brew install --cask bruno

# Open collection
# File → Open Collection → select the bruno/ folder
```

Set the `local` environment — `base_url` is pre-configured to `http://localhost:8080`.
Run `Issue Token` once — the token is captured automatically into `{{token}}` for all
subsequent requests.

---

## API Reference

| Method  | Endpoint        | Auth | Description                                                                        |
| ------- | --------------- | ---- | ---------------------------------------------------------------------------------- |
| `POST`  | `/auth/token`   | None | Issue a signed JWT token (24hr expiry)                                             |
| `POST`  | `/sync`         | JWT  | Diff watchlist against MAL + apply updates · supports `dry_run`                    |
| `PATCH` | `/anime/:id`    | JWT  | Update single entry · auto-fills episodes when `status: completed`                 |
| `GET`   | `/anime/:id`    | JWT  | Full anime details — title, synopsis, genres, themes, rating, studios              |
| `GET`   | `/anime/search` | JWT  | Search MAL by `q` · filter by `genre`, `status`, `type`                            |
| `GET`   | `/list`         | JWT  | User's MAL list · filter by `status`, `type`, `score` · sort by `title` or `score` |

### Request / Response Examples

**POST /sync**

```json
// request
{ "watchlist": [{ "mal_id": 1535, "watchListType": 5 }], "dry_run": true }

// response
{ "total": 1, "succeeded": 0, "failed": 0, "dry_run": true }
```

**PATCH /anime/:id**

```json
// request
{ "status": "completed" }

// response — episodes auto-filled from MAL's total
{ "id": 1535, "updated": true, "episodes_set": 37 }
```

**GET /list**

```
GET /list?status=watching&sort=title&score=7
```

```json
{ "total": 12, "data": [...] }
```

---

## Project Structure

```
mal-updater/
├── cmd/
│   ├── main.go              ← CLI entry point
│   └── server/
│       └── main.go          ← HTTP server entry point
├── auth/                    ← OAuth2 + PKCE — public package
│   ├── pkce.go
│   ├── callback.go
│   ├── browser.go
│   ├── exchange.go
│   └── refresh.go
├── token/                   ← Token struct · Save · Load · IsExpired
│   └── token.go
├── internal/
│   ├── config/              ← Constants — endpoints, ports, concurrency caps
│   ├── session/             ← Token lifecycle orchestration — LoadOrRefresh()
│   ├── diff/                ← Watchlist loader + diff engine
│   ├── mal/                 ← MAL v2 API client + pagination
│   ├── updater/             ← Concurrent batch PATCH
│   └── server/              ← HTTP router, middleware, handlers
│       ├── router.go
│       ├── middleware/
│       │   └── jwt.go
│       └── handlers/
│           ├── handlers.go  ← Handlers struct + shared helpers
│           ├── auth.go      ← POST /auth/token
│           ├── sync.go      ← POST /sync
│           ├── anime.go     ← GET /anime/:id · GET /anime/search
│           ├── list.go      ← GET /list
│           └── update.go    ← PATCH /anime/:id
├── bruno/                   ← Bruno API collection
│   ├── auth/
│   ├── anime/
│   ├── list/
│   ├── sync/
│   ├── update/
│   └── environments/
│       └── local.yml
├── docs/                    ← Technical documentation
├── watchlist.json           ← Your watchlist (gitignored)
├── watchlist.example.json
├── .env                     ← Credentials (gitignored)
├── .env.example
├── .dockerignore            ← Excludes secrets + tooling from Docker build context
├── Dockerfile               ← Two-stage build — golang:1.26-alpine → alpine:3.19
├── docker-compose.yml       ← Port binding · env injection · volume mounts
├── go.mod
└── go.sum
```

---

## Tech

- Go standard library — `net/http`, `crypto/rand`, `encoding/json`, `sync`, `flag`
- [`godotenv`](https://github.com/joho/godotenv) — `.env` file loading
- [`golang-jwt/jwt`](https://github.com/golang-jwt/jwt) — JWT signing + validation
- [`go-chi/chi`](https://github.com/go-chi/chi) — lightweight HTTP router with URL params
- [Docker](https://www.docker.com) — two-stage build, ~15MB runtime image
- [Bruno](https://www.usebruno.com) — API collection (git-friendly, no account required)

---

## Phase Progress

| Phase | Description                                                | Status |
| ----- | ---------------------------------------------------------- | ------ |
| 1     | Environment setup + PKCE generation                        | ✅     |
| 2     | OAuth2 callback server                                     | ✅     |
| 3     | Token exchange + storage                                   | ✅     |
| 4     | MAL API client + pagination                                | ✅     |
| 5     | Watchlist loader (multi-format)                            | ✅     |
| 6     | Diff engine                                                | ✅     |
| 7     | Concurrent batch updater                                   | ✅     |
| 8     | Silent token refresh                                       | ✅     |
| 9     | `--dry-run` CLI flag                                       | ✅     |
| 10    | Structural refactor — `auth/`, `token/`, `session/`        | ✅     |
| 11    | HTTP server + JWT middleware + handlers + Bruno collection | ✅     |
| 12    | Docker — two-stage build + compose                         | ✅     |

---

## License

[MIT](./LICENSE)
