# mal-updater

Automate MyAnimeList updates from a local watchlist file — built in Go.

---

## What It Does

`mal-updater` is a CLI tool and HTTP/gRPC API that reads your anime watchlist from a local
`watchlist.json` file, compares it against your live MyAnimeList account,
and PATCHes only the entries that differ — concurrently.

No manual MAL updates. No full list replacements. Just the delta.

**Three modes:**

- **CLI** — run locally, sync on demand
- **HTTP Server** — JWT-protected REST API, consumable via curl or Bruno
- **gRPC Server** — binary Protocol Buffer API, consumable as a microservice

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
GRPC_PORT=9090
```

### Add Your Watchlist

Export your watchlist from HiAnime and save it as `watchlist.json` in the project root.

---

## Usage

### CLI

```bash
go run cmd/main.go           # full sync
go run cmd/main.go --dry-run # preview only
```

### HTTP Server

```bash
go run cmd/server/main.go
# → Server running on :8080
```

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/auth/token | jq -r .token)

curl http://localhost:8080/anime/1535 -H "Authorization: Bearer $TOKEN"
curl "http://localhost:8080/list?status=watching" -H "Authorization: Bearer $TOKEN"
```

### gRPC Server

```bash
go run cmd/grpc/main.go
# → gRPC server running on :9090
```

```bash
# Test with grpcurl
brew install grpcurl

grpcurl -plaintext localhost:9090 list
grpcurl -plaintext -d '{"id": "1535"}' localhost:9090 anime.AnimeService/GetAnime
grpcurl -plaintext -d '{"q": "naruto"}' localhost:9090 anime.AnimeService/Search
grpcurl -plaintext -d '{"status": "watching"}' localhost:9090 anime.AnimeService/GetList
```

### Regenerate Proto Code

```bash
make proto
```

### Docker

```bash
go run cmd/main.go          # authenticate first — creates token.json
docker compose up           # start HTTP server
docker compose down
```

### Bruno (API Collection)

```bash
brew install --cask bruno
# File → Open Collection → select bruno/ folder
```

---

## API Reference

### HTTP (REST)

| Method  | Endpoint        | Auth | Description                                             |
| ------- | --------------- | ---- | ------------------------------------------------------- |
| `POST`  | `/auth/token`   | None | Issue a signed JWT token (24hr expiry)                  |
| `POST`  | `/sync`         | JWT  | Diff watchlist against MAL + apply updates              |
| `PATCH` | `/anime/:id`    | JWT  | Update single entry · auto-fills episodes on completed  |
| `GET`   | `/anime/:id`    | JWT  | Full anime details                                      |
| `GET`   | `/anime/search` | JWT  | Search MAL by `q` · filter by `genre`, `status`, `type` |
| `GET`   | `/list`         | JWT  | User's MAL list with filters                            |

### gRPC (Protocol Buffers)

Service: `anime.AnimeService` · Port: `:9090`

| RPC        | Request                                               | Response              | Description        |
| ---------- | ----------------------------------------------------- | --------------------- | ------------------ |
| `GetAnime` | `GetAnimeRequest{id}`                                 | `AnimeResponse`       | Full anime details |
| `Search`   | `SearchAnimeRequest{q, genre, status, media_type}`    | `SearchAnimeResponse` | Search anime       |
| `GetList`  | `GetListRequest{status, media_type, sort, min_score}` | `GetListResponse`     | User's MAL list    |

---

## Project Structure

```
mal-updater/
├── cmd/
│   ├── main.go              ← CLI entry point
│   ├── server/
│   │   └── main.go          ← HTTP server entry point (:8080)
│   └── grpc/
│       └── main.go          ← gRPC server entry point (:9090)
├── proto/
│   ├── anime.proto          ← gRPC contract — source of truth
│   └── animepb/
│       ├── anime.pb.go      ← generated message types
│       └── anime_grpc.pb.go ← generated service + client stub
├── auth/                    ← OAuth2 + PKCE — public package
├── token/                   ← Token struct · Save · Load · IsExpired
├── internal/
│   ├── config/
│   ├── session/             ← Token lifecycle — LoadOrRefresh()
│   ├── diff/                ← Watchlist loader + diff engine
│   ├── mal/                 ← MAL v2 API client + pagination
│   ├── updater/             ← Concurrent batch PATCH
│   ├── grpc/
│   │   └── server.go        ← AnimeServer gRPC handler implementation
│   └── server/              ← HTTP router, middleware, handlers
├── bruno/                   ← Bruno API collection
├── Makefile                 ← make proto
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

---

## Tech

- Go standard library — `net/http`, `crypto/rand`, `encoding/json`, `sync`, `flag`
- [`godotenv`](https://github.com/joho/godotenv) — `.env` file loading
- [`golang-jwt/jwt`](https://github.com/golang-jwt/jwt) — JWT signing + validation
- [`go-chi/chi`](https://github.com/go-chi/chi) — HTTP router
- [`google.golang.org/grpc`](https://pkg.go.dev/google.golang.org/grpc) — gRPC framework
- [`google.golang.org/protobuf`](https://pkg.go.dev/google.golang.org/protobuf) — Protocol Buffers
- [Docker](https://www.docker.com) — two-stage build, ~15MB runtime image
- [Bruno](https://www.usebruno.com) — HTTP API collection

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
| 11    | HTTP server + JWT middleware + handlers + Bruno     | ✅     |
| 12    | Docker — two-stage build + compose                  | ✅     |
| 13    | gRPC server — proto contract + AnimeService         | ✅     |

---

## License

[MIT](./LICENSE)
