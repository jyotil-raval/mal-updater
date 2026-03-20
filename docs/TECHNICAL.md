# MAL Updater — Technical Documentation

> Architecture details, data flow, and implementation notes for the `mal-updater` project.

---

## Table of Contents

- [Project Overview](#project-overview)
- [Package Structure](#package-structure)
- [Architecture](#architecture)
- [Data Flow](#data-flow)
- [HTTP API](#http-api)
- [gRPC API](#grpc-api)
- [Docker](#docker)
- [External Dependencies](#external-dependencies)
- [Configuration Reference](#configuration-reference)
- [Critical Implementation Notes](#critical-implementation-notes)

---

## Project Overview

| Field                 | Value                                                                                                               |
| --------------------- | ------------------------------------------------------------------------------------------------------------------- |
| Language              | Go 1.26 · darwin/arm64 (Apple Silicon)                                                                              |
| Module                | `github.com/jyotil-raval/mal-updater`                                                                               |
| External dependencies | `godotenv v1.5.1` · `golang-jwt/jwt v5` · `go-chi/chi v5` · `google.golang.org/grpc` · `google.golang.org/protobuf` |
| Status                | Phase 13 complete                                                                                                   |

**Purpose:** CLI tool, HTTP API, and gRPC API that synchronises a locally exported HiAnime watchlist with a MyAnimeList (MAL) account. Exposes a JWT-protected REST API and a gRPC service consumable as a microservice by Project 2.

---

## Package Structure

```
mal-updater/
├── cmd/
│   ├── main.go                  # CLI entry point
│   ├── server/
│   │   └── main.go              # HTTP server (:8080)
│   └── grpc/
│       └── main.go              # gRPC server (:9090)
├── proto/
│   ├── anime.proto              # gRPC contract — source of truth
│   └── animepb/
│       ├── anime.pb.go          # generated — message types
│       └── anime_grpc.pb.go     # generated — service interface + client stub
├── auth/                        # OAuth2 + PKCE — public package
├── token/                       # Token struct · Save · Load · IsExpired
├── internal/
│   ├── config/
│   │   └── constants.go
│   ├── session/
│   │   └── session.go           # Token lifecycle — LoadOrRefresh()
│   ├── diff/
│   │   ├── types.go
│   │   ├── loader.go
│   │   └── engine.go
│   ├── mal/
│   │   ├── types.go
│   │   └── client.go
│   ├── updater/
│   │   ├── patch.go
│   │   └── batch.go
│   ├── grpc/
│   │   └── server.go            # AnimeServer — implements AnimeServiceServer
│   └── server/
│       ├── router.go
│       ├── middleware/
│       │   └── jwt.go
│       └── handlers/
│           ├── handlers.go
│           ├── auth.go
│           ├── sync.go
│           ├── anime.go
│           ├── list.go
│           └── update.go
├── bruno/
├── Makefile                     # make proto
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

---

## Architecture

### Package Responsibilities

| Package              | Key Files                                    | Responsibility                                                   |
| -------------------- | -------------------------------------------- | ---------------------------------------------------------------- |
| `cmd/main.go`        | `main.go`                                    | CLI · `--dry-run` · orchestration                                |
| `cmd/server/main.go` | `main.go`                                    | HTTP server · :8080                                              |
| `cmd/grpc/main.go`   | `main.go`                                    | gRPC server · :9090                                              |
| `auth/`              | `pkce, callback, browser, exchange, refresh` | OAuth2 PKCE flow · public package                                |
| `token/`             | `token.go`                                   | Token struct · Save/Load · expiry · public package               |
| `internal/session`   | `session.go`                                 | Token lifecycle — loads, refreshes, or re-authenticates          |
| `internal/config`    | `constants.go`                               | All global constants                                             |
| `internal/mal`       | `types.go, client.go`                        | MAL API client · offset pagination                               |
| `internal/diff`      | `types.go, loader.go, engine.go`             | Format detection · watchlist parsing · diff                      |
| `internal/updater`   | `patch.go, batch.go`                         | Concurrent batch PATCH · semaphore                               |
| `internal/server`    | `router, middleware, handlers`               | Chi router · JWT middleware · HTTP handlers                      |
| `internal/grpc`      | `server.go`                                  | gRPC handler implementations                                     |
| `proto/anime.proto`  | —                                            | Contract definition — source of truth for both server and client |
| `proto/animepb/`     | `anime.pb.go, anime_grpc.pb.go`              | Generated Go code — never edit manually                          |

### Three Entry Points

```
go run cmd/main.go          ← CLI sync
go run cmd/server/main.go   ← HTTP server :8080
go run cmd/grpc/main.go     ← gRPC server :9090
```

All three share `internal/session.LoadOrRefresh()` for token lifecycle. All three call the same `internal/mal`, `internal/diff`, and `internal/updater` packages.

### HTTP vs gRPC — When to Use Which

|                 | HTTP (REST)                    | gRPC                             |
| --------------- | ------------------------------ | -------------------------------- |
| Format          | JSON — human readable          | Protobuf — binary, smaller       |
| Browser support | Native                         | Needs proxy                      |
| Contract        | Implicit (docs/Bruno)          | Explicit (.proto file)           |
| Best for        | User-facing, curl, Bruno       | Service-to-service (media-shelf) |
| Auth            | `Authorization: Bearer` header | gRPC metadata                    |
| Port            | 8080                           | 9090                             |

### Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│  External / File System                                                  │
│  .env · token.json · watchlist.json                                      │
└─────────────────────────────┬────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────┐  ┌───────────────────┐  ┌──────────────────────────────┐
│ cmd/main.go  │  │ cmd/server/main.go│  │ cmd/grpc/main.go             │
│ CLI          │  │ HTTP :8080        │  │ gRPC :9090                   │
└──────┬───────┘  └────────┬──────────┘  └──────────────┬───────────────┘
       │                   │                             │
       └───────────────────┼─────────────────────────────┘
                           ▼
              ┌────────────────────────┐
              │   internal/session     │
              │   LoadOrRefresh()      │
              └──────┬─────────────┬───┘
                     │             │
                     ▼             ▼
                ┌─────────┐  ┌──────────┐
                │  auth/  │  │  token/  │
                └─────────┘  └──────────┘

  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐  ┌──────────────┐
  │ internal │  │ internal │  │ internal │  │  internal   │  │   internal   │
  │ /config  │  │ /diff    │  │ /mal     │  │  /updater   │  │   /grpc      │
  │          │  │          │  │          │  │             │  │  /server     │
  │ Constants│  │ Loader   │  │ API      │  │ Patch       │  │ AnimeServer  │
  │ Endpoints│  │ Engine   │  │ Client   │  │ Batch       │  │ HTTP handlers│
  └──────────┘  └──────────┘  └────┬─────┘  └──────┬──────┘  └──────────────┘
                                   │               │
                                   ▼               ▼
                            ┌─────────────────────────────┐
                            │   External — MAL APIs        │
                            │   OAuth2 · REST API          │
                            └─────────────────────────────┘
```

---

## Data Flow

```
┌─────────────────────────────────────────────────────┐
│  1 · Entry Point                                     │
│  CLI / HTTP / gRPC server starts                     │
│  godotenv.Load() · validate env vars                 │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│  2 · Token Lifecycle  (internal/session)             │
│  Load → Valid / Expired (refresh) / Missing (auth)   │
└──────────────────────┬──────────────────────────────┘
                       │ access_token
                       ▼
┌─────────────────────────────────────────────────────┐
│  3 · Fetch MAL Anime List (CLI sync path only)       │
│  GET /users/@me/animelist · paginated                │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│  4 · Load Watchlist                                  │
│  CLI: file · HTTP/gRPC: request body                 │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│  5 · Diff + Apply / Query                            │
│  CLI/HTTP POST /sync: diff → batch PATCH             │
│  HTTP/gRPC GET: forward to MAL API → return          │
└──────────────────────────────────────────────────────┘
```

---

## HTTP API

All protected routes require `Authorization: Bearer <token>` header.

| Method  | Endpoint        | Auth | Description                                     |
| ------- | --------------- | ---- | ----------------------------------------------- |
| `POST`  | `/auth/token`   | None | Issue JWT (24hr, HS256)                         |
| `POST`  | `/sync`         | JWT  | Diff + apply · supports `dry_run`               |
| `PATCH` | `/anime/:id`    | JWT  | Update entry · auto-fills episodes on completed |
| `GET`   | `/anime/:id`    | JWT  | Full anime details                              |
| `GET`   | `/anime/search` | JWT  | Search MAL + client-side genre/status filter    |
| `GET`   | `/list`         | JWT  | User's MAL list with filters                    |

---

## gRPC API

**Port:** `:9090` · **Protocol:** Protocol Buffers over TCP
**Reflection:** enabled (development) — allows `grpcurl list`

### Service Definition

```protobuf
service AnimeService {
  rpc GetAnime  (GetAnimeRequest)    returns (AnimeResponse);
  rpc Search    (SearchAnimeRequest) returns (SearchAnimeResponse);
  rpc GetList   (GetListRequest)     returns (GetListResponse);
}
```

### Key Messages

```protobuf
message AnimeResponse {
  string id           = 1;
  string title        = 2;
  string synopsis     = 3;
  string media_type   = 4;
  string status       = 5;
  int32  num_episodes = 6;
  string start_date   = 7;
  string end_date     = 8;
  double mean_score   = 9;
  int32  rank         = 10;
  int32  popularity   = 11;
  string rating       = 12;
  repeated Genre  genres  = 13;
  repeated Studio studios = 14;
}
```

### Regenerating Go Code

```bash
make proto
# runs: protoc --go_out=. --go_opt=module=... --go-grpc_out=. ...
```

**Never edit `proto/animepb/*.go` manually** — they are generated files. Edit `proto/anime.proto` and run `make proto`.

### Testing with grpcurl

```bash
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext -d '{"id": "1535"}' localhost:9090 anime.AnimeService/GetAnime
grpcurl -plaintext -d '{"q": "naruto"}' localhost:9090 anime.AnimeService/Search
grpcurl -plaintext -d '{"status": "watching"}' localhost:9090 anime.AnimeService/GetList
```

---

## Docker

### Two-Stage Build

```
Stage 1 — Builder (golang:1.26-alpine)
  ├── gcc + musl-dev (cgo for go-sqlite3)
  ├── go mod download (cached layer)
  ├── copy source + compile HTTP server binary
  └── -ldflags="-w -s"

Stage 2 — Runtime (alpine:3.19)
  ├── ca-certificates (HTTPS to MAL)
  └── binary only — ~15MB image
```

Note: Docker currently runs the HTTP server only. The gRPC server runs locally via `go run cmd/grpc/main.go`.

### Volume Mounts

| Host             | Container             | Purpose          |
| ---------------- | --------------------- | ---------------- |
| `token.json`     | `/app/token.json`     | MAL access token |
| `watchlist.json` | `/app/watchlist.json` | Local watchlist  |

---

## External Dependencies

### MAL OAuth2 · `https://myanimelist.net/v1/oauth2/`

| Endpoint     | Method           | Purpose            |
| ------------ | ---------------- | ------------------ |
| `/authorize` | Browser redirect | PKCE auth flow     |
| `/token`     | POST             | Auth code → tokens |
| `/token`     | POST             | Refresh token      |

### MAL REST API · `https://api.myanimelist.net/v2/`

| Endpoint                     | Method | Purpose         |
| ---------------------------- | ------ | --------------- |
| `/users/@me/animelist`       | GET    | Full anime list |
| `/anime/{id}`                | GET    | Anime details   |
| `/anime`                     | GET    | Search          |
| `/anime/{id}/my_list_status` | PATCH  | Update entry    |

---

## Configuration Reference

| Constant               | Value                                         |
| ---------------------- | --------------------------------------------- |
| `MALAuthURL`           | `https://myanimelist.net/v1/oauth2/authorize` |
| `MALTokenURL`          | `https://myanimelist.net/v1/oauth2/token`     |
| `MALAPIBaseURL`        | `https://api.myanimelist.net/v2`              |
| `CallbackPort`         | `8080`                                        |
| `PKCEMethod`           | `plain`                                       |
| `TokenExpireBuffer`    | `5` (minutes)                                 |
| `MALListLimit`         | `1000`                                        |
| `MALUpdateConcurrency` | `3`                                           |

Environment variables:

| Variable           | Purpose                   |
| ------------------ | ------------------------- |
| `MAL_CLIENT_ID`    | MAL API client ID         |
| `MAL_REDIRECT_URI` | OAuth2 callback URL       |
| `JWT_SECRET`       | JWT signing secret        |
| `SERVER_PORT`      | HTTP port (default: 8080) |
| `GRPC_PORT`        | gRPC port (default: 9090) |

---

## Critical Implementation Notes

**MAL forces plain PKCE** — `S256` returns `400 invalid_grant`.

**GET vs PATCH field names** — `num_episodes_watched` (GET) vs `num_watched_episodes` (PATCH).

**Episode auto-fill** — `completed` status triggers fetching `num_episodes` from MAL.

**Token expiry buffer** — `IsExpired()` fires 5 minutes early.

**Semaphore via buffered channel**

```go
sem := make(chan struct{}, 3)
sem <- struct{}{}; defer func() { <-sem }()
```

**Circular import resolution** — `auth/` → `token/`. `session/` orchestrates both. Acyclic.

**gRPC reflection** — enabled for development only. Disable in production to hide API surface.

**Proto field numbers are permanent** — changing a field number breaks all serialised data. Add new fields with new numbers. Never renumber.

**Never edit generated proto files** — `proto/animepb/*.go` are generated by `protoc`. Edit `proto/anime.proto` and run `make proto`.

**`float64` from `map[string]any`** — JSON numbers decode as `float64` in generic maps. Always cast: `int32(data["num_episodes"].(float64))`.

**gRPC package name collision**

```go
grpcserver "github.com/jyotil-raval/mal-updater/internal/grpc" // your package
"google.golang.org/grpc"                                         // the library
```

The alias disambiguates your implementation from the gRPC library.

**`godotenv.Load()` non-fatal** — Docker injects env vars directly; no `.env` in container.

---

_MAL Updater · Technical Documentation · March 2026_
