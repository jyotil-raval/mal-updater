# MAL Updater — Technical Documentation

> Architecture details, data flow, and implementation notes for the `mal-updater` project.

---

## Table of Contents

- [Project Overview](#project-overview)
- [Package Structure](#package-structure)
- [Architecture](#architecture)
- [Data Flow](#data-flow)
- [External Dependencies](#external-dependencies)
- [Configuration Reference](#configuration-reference)
- [Critical Implementation Notes](#critical-implementation-notes)

---

## Project Overview

| Field                 | Value                                                                         |
| --------------------- | ----------------------------------------------------------------------------- |
| Language              | Go 1.26 · darwin/arm64 (Apple Silicon)                                        |
| Module                | `github.com/jyotil-raval/mal-updater`                                         |
| External dependencies | `github.com/joho/godotenv v1.5.1` · `github.com/golang-jwt/jwt/v5` (Phase 11) |
| Status                | Phase 10 complete — HTTP server + Docker in progress                          |

**Purpose:** CLI tool and HTTP API that synchronises a locally exported HiAnime watchlist with a MyAnimeList (MAL) account. Handles OAuth2 authentication, reads the local watchlist in multiple formats, computes the diff against the current MAL state, and applies required updates concurrently via the MAL REST API. Exposes a JWT-protected REST API consumable as a microservice by Project 2.

---

## Package Structure

```
mal-updater/
├── cmd/
│   ├── main.go                  # CLI entry point · flags · orchestration
│   └── server/
│       └── main.go              # HTTP server entry point (Phase 11)
├── auth/                        # OAuth2 + PKCE — public package
│   ├── pkce.go                  # PKCE pair generation (plain method)
│   ├── callback.go              # Temporary HTTP server for OAuth2 callback
│   ├── browser.go               # Cross-platform browser launch
│   ├── exchange.go              # Authorization code → token exchange
│   └── refresh.go               # Silent refresh token exchange
├── token/                       # Token struct · Save · Load · IsExpired
│   └── token.go
├── internal/
│   ├── config/
│   │   └── constants.go         # All global constants — URLs, ports, caps
│   ├── session/
│   │   └── session.go           # Token lifecycle orchestration — LoadOrRefresh()
│   ├── diff/
│   │   ├── types.go             # Update, WatchlistEntry structs
│   │   ├── loader.go            # Format detection · Watchlist parsing
│   │   └── engine.go            # Status diff computation
│   ├── mal/
│   │   ├── types.go             # ListEntry, Node, ListStatus structs
│   │   └── client.go            # MAL API client · Offset pagination
│   ├── updater/
│   │   ├── patch.go             # Single-entry PATCH call
│   │   └── batch.go             # Concurrent batch runner · Semaphore + WaitGroup
│   └── server/                  # HTTP router, middleware, handlers (Phase 11)
│       ├── router.go
│       ├── middleware/
│       │   └── jwt.go
│       └── handlers/
│           ├── auth.go
│           ├── sync.go
│           ├── anime.go
│           ├── list.go
│           └── update.go
├── docs/
├── watchlist.json               # gitignored — local HiAnime export
├── watchlist.example.json
├── .env                         # gitignored
├── .env.example
├── Dockerfile                   # Phase 12
└── docker-compose.yml           # Phase 12
```

---

## Architecture

### Package Responsibilities

| Package              | Key Files                                    | Responsibility                                                                  |
| -------------------- | -------------------------------------------- | ------------------------------------------------------------------------------- |
| `cmd/main.go`        | `main.go`                                    | CLI entry point · `--dry-run` flag · orchestration                              |
| `cmd/server/main.go` | `main.go`                                    | HTTP server entry point · port binding (Phase 11)                               |
| `auth/`              | `pkce, callback, browser, exchange, refresh` | Full OAuth2 PKCE flow · public package                                          |
| `token/`             | `token.go`                                   | Token struct · Save/Load to disk · expiry check (5-min buffer) · public package |
| `internal/session`   | `session.go`                                 | Token lifecycle orchestration — loads, refreshes, or re-authenticates           |
| `internal/config`    | `constants.go`                               | All global constants — API URLs, port, PKCE method, concurrency cap             |
| `internal/mal`       | `types.go, client.go`                        | MAL API client · offset pagination · typed response structs                     |
| `internal/diff`      | `types.go, loader.go, engine.go`             | Format detection · watchlist parsing · status diff computation                  |
| `internal/updater`   | `patch.go, batch.go`                         | Single-entry PATCH · concurrent batch runner · semaphore + WaitGroup            |
| `internal/server`    | `router, middleware, handlers`               | HTTP routing · JWT validation · request handlers (Phase 11)                     |

### Why `auth/` and `token/` are public packages

Both packages are outside `internal/` so they can be imported by `media-shelf` (Project 2) as an external module dependency. `internal/` enforces a compile-time boundary — packages inside it are invisible to code outside the module. Moving auth and token out makes them reusable across projects.

### Why `internal/session/` exists

`session.go` orchestrates `auth` and `token` together — it calls `token.Load()`, `auth.RefreshToken()`, and `auth.ExchangeCode()`. Placing this logic in either `auth/` or `token/` would create a circular import:

```
auth → imports → token        ✅
token → imports → auth        ❌ circular
```

`internal/session` sits above both — importing `auth` and `token` without being imported by either. Both `cmd/main.go` and `cmd/server/main.go` call `session.LoadOrRefresh()` — one call, shared lifecycle.

### Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────┐
│  External / File System                                              │
│                                                                      │
│  ┌──────────────┐  ┌─────────────┐  ┌───────────────┐                │
│  │    .env      │  │ token.json  │  │watchlist.json │                │
│  │ Client ID    │  │ Access +    │  │ Local anime   │                │
│  │ JWT Secret   │  │ Refresh     │  │ list          │                │
│  └──────┬───────┘  └──────┬──────┘  └───────┬───────┘                │
└─────────┼─────────────────┼─────────────────┼────────────────────────┘
          │                 │                 │
          ▼                 ▼                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│   cmd/main.go                   cmd/server/main.go  (Phase 11)      │
│   CLI · --dry-run               HTTP Server · JWT · Handlers        │
└──────┬──────────────────────────────────┬───────────────────────────┘
       │                                  │
       └──────────────┬───────────────────┘
                      ▼
          ┌───────────────────────┐
          │  internal/session     │
          │  LoadOrRefresh()      │
          └────┬─────────────┬────┘
               │             │
               ▼             ▼
          ┌─────────┐   ┌──────────┐
          │  auth/  │   │  token/  │
          │  PKCE   │   │  CRUD    │
          │  OAuth2 │   │  Expiry  │
          └─────────┘   └──────────┘

       ┌────────────┐  ┌──────────────┐  ┌─────────────┐
       │  internal  │  │   internal   │  │  internal   │
       │  /config   │  │   /diff      │  │  /mal       │
       │            │  │              │  │             │
       │ Constants  │  │ Loader       │  │ API Client  │
       │ Endpoints  │  │ Engine       │  │ Pagination  │
       └────────────┘  └──────┬───────┘  └──────┬──────┘
                              │                 │
                              ▼                 ▼
                       ┌─────────────┐   ┌─────────────┐
                       │  internal   │   │  External   │
                       │  /updater   │   │  MAL APIs   │
                       │             │   │             │
                       │ Patch       │   │ OAuth2      │
                       │ Batch       │   │ REST API    │
                       │ Semaphore   │   └─────────────┘
                       └─────────────┘
```

---

## Data Flow

### Flow Diagram

```
┌──────────────────────────────────────────────────────┐
│  1 · Entry Point                                     │
│                                                      │
│  CLI:    go run cmd/main.go [--dry-run]              │
│  Server: go run cmd/server/main.go                   │
│                                                      │
│  flag.Parse() · godotenv.Load() · validate env vars  │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  2 · Token Lifecycle  (internal/session)             │
│                                                      │
│  token.Load(token.json)                              │
│    ├── Valid?      ──────────────────► Use directly  │
│    ├── Expired?    ──► RefreshToken() ──► Save ──►   │
│    └── Not found? ──► Full Auth Flow ──► Save ──►    │
│                                                      │
│  Full Auth: PKCE gen → Browser → Callback server     │
│             → Code exchange → token.json             │
└──────────────────────┬───────────────────────────────┘
                       │ access_token
                       ▼
┌──────────────────────────────────────────────────────┐
│  3 · Fetch MAL Anime List                            │
│                                                      │
│  GET /users/@me/animelist                            │
│  fields: list_status, num_episodes                   │
│  Paginated (limit=1000, offset-based)                │
│  → []mal.ListEntry                                   │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  4 · Load Local Watchlist  (CLI only)                │
│                                                      │
│  diff.LoadWatchlist("watchlist.json")                │
│                                                      │
│  First byte == '['  ──► loadFlatArray()              │
│  First byte == '{'  ──► loadCategorized()            │
│                                                      │
│  Both paths → []diff.WatchlistEntry                  │
│                                                      │
│  HTTP: watchlist supplied in POST /sync request body │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│  5 · Compare (Diff Engine)                          │
│                                                     │
│  Build map[animeID]mal.ListEntry from MAL data      │
│  For each local entry:                              │
│    · Status mismatch?        → queue update         │
│    · Status == completed &&  → set episodes to      │
│      mal.NumEpisodes > 0       series total         │
│                                                     │
│  → []diff.Update  (N entries requiring change)      │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
              ┌────────┴────────┐
              │  updates == 0?  │
              └────┬──────┬─────┘
                  YES      NO
                   │        │
                   ▼        ▼
           ┌──────────┐  ┌───────────────────┐
           │ Already  │  │  --dry-run set?   │
           │ in sync  │  └────┬─────────┬────┘
           │ Exit     │      YES       NO
           └──────────┘       │         │
                              ▼         ▼
                    ┌─────────────┐ ┌──────────────────────────────────┐
                    │ Print each  │ │  7 · Batch PATCH (Concurrent)    │
                    │ planned     │ │                                  │
                    │ update      │ │  ApplyUpdates(updates, token)    │
                    │ Exit        │ │  Goroutine per update            │
                    └─────────────┘ │  Buffered channel semaphore (3)  │
                                    │  sync.WaitGroup + sync.Mutex     │
                                    │                                  │
                                    │  PATCH /anime/{id}/my_list_status│
                                    └──────────────┬───────────────────┘
                                                   │
                                                   ▼
                                    ┌──────────────────────────────────┐
                                    │  Done                            │
                                    │  N succeeded · M failed · Exit   │
                                    └──────────────────────────────────┘
```

### Step-by-Step Description

**1 · Entry Point**
CLI: `flag.Parse()` reads `--dry-run`. Server: binds to `SERVER_PORT`. Both load `.env` and validate `MAL_CLIENT_ID` and `MAL_REDIRECT_URI`.

**2 · Token Lifecycle**
`session.LoadOrRefresh()` handles three paths — valid token, expired token (silent refresh), no token (full PKCE browser flow). Both CLI and server call this single function.

**3 · Fetch MAL Anime List**
`mal.GetAnimeList()` pages through `/users/@me/animelist` using offset-based pagination with `limit=1000`. Requests `fields=list_status,num_episodes`.

**4 · Load Local Watchlist**
CLI reads `watchlist.json` — format auto-detected by first byte (`[` vs `{`). HTTP server receives watchlist in the `POST /sync` request body.

**5 · Compare (Diff Engine)**
`diff.Compare()` builds a `map[int]mal.ListEntry` keyed by anime ID. Queues updates for status mismatches. Auto-fills episodes to series total for completed titles.

**6 · Decision Branch**
Zero updates → exit. Dry run → print and exit. Otherwise → batch updater.

**7 · Concurrent Batch PATCH**
`updater.ApplyUpdates()` — one goroutine per update, buffered channel semaphore (cap 3), `sync.WaitGroup` + `sync.Mutex`.

---

## HTTP API _(Phase 11)_

| Method  | Endpoint        | Auth | Description                                            |
| ------- | --------------- | ---- | ------------------------------------------------------ |
| `POST`  | `/auth/token`   | None | Issue a JWT token                                      |
| `POST`  | `/sync`         | JWT  | Full watchlist diff + apply                            |
| `PATCH` | `/anime/:id`    | JWT  | Update single entry — auto-fills episodes on completed |
| `GET`   | `/anime/:id`    | JWT  | Full anime details from MAL                            |
| `GET`   | `/anime/search` | JWT  | Search MAL + filter local list                         |
| `GET`   | `/list`         | JWT  | User's MAL list with filters                           |

> Full handler documentation will be added when Phase 11 is complete.

---

## External Dependencies

### MAL OAuth2 Server

`https://myanimelist.net/v1/oauth2/`

| Endpoint     | Method           | Purpose                                     |
| ------------ | ---------------- | ------------------------------------------- |
| `/authorize` | Browser redirect | Initiate PKCE auth flow                     |
| `/token`     | POST             | Exchange auth code for tokens               |
| `/token`     | POST             | Exchange refresh token for new access token |

### MAL REST API

`https://api.myanimelist.net/v2/`

| Endpoint                     | Method | Purpose                                                   |
| ---------------------------- | ------ | --------------------------------------------------------- |
| `/users/@me/animelist`       | GET    | Fetch full anime list with status + episode data          |
| `/anime/{id}`                | GET    | Full anime details — genres, themes, demographics, rating |
| `/anime`                     | GET    | Search anime by query + filters                           |
| `/anime/{id}/my_list_status` | PATCH  | Update a single entry's status and/or episode count       |

---

## Configuration Reference

All constants live in `internal/config/constants.go`.

| Constant               | Value                                         |
| ---------------------- | --------------------------------------------- |
| `MALAuthURL`           | `https://myanimelist.net/v1/oauth2/authorize` |
| `MALTokenURL`          | `https://myanimelist.net/v1/oauth2/token`     |
| `MALAPIBaseURL`        | `https://api.myanimelist.net/v2`              |
| `CallbackPort`         | `8080`                                        |
| `CallbackPath`         | `/callback`                                   |
| `PKCEMethod`           | `plain`                                       |
| `GrantTypeAuth`        | `authorization_code`                          |
| `GrantTypeRefresh`     | `refresh_token`                               |
| `TokenFile`            | `token.json`                                  |
| `TokenExpireBuffer`    | `5` (minutes)                                 |
| `MALListLimit`         | `1000`                                        |
| `MALUpdateConcurrency` | `3`                                           |

Environment variables (`.env`):

| Variable           | Purpose                                  |
| ------------------ | ---------------------------------------- |
| `MAL_CLIENT_ID`    | MAL API client ID                        |
| `MAL_REDIRECT_URI` | OAuth2 callback URL                      |
| `JWT_SECRET`       | Signing secret for JWT tokens (Phase 11) |
| `SERVER_PORT`      | HTTP server port (Phase 11)              |

---

## Critical Implementation Notes

**MAL forces plain PKCE**
Setting `code_challenge_method=S256` returns `400 invalid_grant: Failed to verify code_verifier`. `PKCEMethod` is fixed to `plain` — challenge equals verifier.

**GET vs PATCH field name asymmetry**
The MAL API uses different field names for reading and writing episode counts:

- GET response → `num_episodes_watched`
- PATCH body → `num_watched_episodes`

**Episode count logic**
Episodes are never read from `watchlist.json` — HiAnime exports carry no episode data. For `completed` titles, MAL's own `num_episodes` is used as the target. The same rule applies to `PATCH /anime/:id` — sending `status: completed` auto-fills episodes from MAL's total.

**Token expiry buffer**
`token.IsExpired()` returns `true` 5 minutes before actual expiry to avoid races on token use mid-request.

**Token file permissions**
`token.json` is written with `0600` (owner read/write only).

**Semaphore via buffered channel**

```go
sem := make(chan struct{}, MALUpdateConcurrency) // capacity = 3
sem <- struct{}{}  // acquire
// ... HTTP call ...
<-sem              // release
```

**Circular import resolution**
`auth/` imports `token/`. `token/` imports nothing internal. `internal/session/` imports both — it is the only package allowed to orchestrate them together. No other package imports `session`.

**Variable shadowing — `token` package vs `token` variable**
The package is named `token`. Local variables that hold a `token.Token` value must use a different name (e.g. `tok`) to avoid shadowing the package name and breaking subsequent `token.Save()` / `token.Load()` calls.

---

_MAL Updater · Technical Documentation · March 2026_
