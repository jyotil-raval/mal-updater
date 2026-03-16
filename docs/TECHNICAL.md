# MAL Updater — Technical Documentation

> Architecture details, data flow, and implementation notes for the `mal-updater` project.

---

## Table of Contents

- [Project Overview](#project-overview)
- [Package Structure](#package-structure)
- [Architecture](#architecture)
- [Data Flow](#data-flow)
- [HTTP API](#http-api)
- [External Dependencies](#external-dependencies)
- [Configuration Reference](#configuration-reference)
- [Critical Implementation Notes](#critical-implementation-notes)

---

## Project Overview

| Field                 | Value                                                     |
| --------------------- | --------------------------------------------------------- |
| Language              | Go 1.26 · darwin/arm64 (Apple Silicon)                    |
| Module                | `github.com/jyotil-raval/mal-updater`                     |
| External dependencies | `godotenv v1.5.1` · `golang-jwt/jwt v5` · `go-chi/chi v5` |
| Status                | Phase 11 complete — Docker in progress                    |

**Purpose:** CLI tool and HTTP API that synchronises a locally exported HiAnime watchlist with a MyAnimeList (MAL) account. Handles OAuth2 authentication, reads the local watchlist in multiple formats, computes the diff against the current MAL state, and applies required updates concurrently via the MAL REST API. Exposes a JWT-protected REST API consumable as a microservice by Project 2.

---

## Package Structure

```
mal-updater/
├── cmd/
│   ├── main.go                  # CLI entry point · flags · orchestration
│   └── server/
│       └── main.go              # HTTP server entry point
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
│   └── server/
│       ├── router.go            # Chi router · route registration · middleware wiring
│       ├── middleware/
│       │   └── jwt.go           # JWT validation · attaches claims to context
│       └── handlers/
│           ├── handlers.go      # Handlers struct · writeJSON · writeError helpers
│           ├── auth.go          # POST /auth/token
│           ├── sync.go          # POST /sync
│           ├── anime.go         # GET /anime/:id · GET /anime/search
│           ├── list.go          # GET /list
│           └── update.go        # PATCH /anime/:id
├── bruno/                       # Bruno API collection — git-friendly, no account needed
│   ├── auth/
│   ├── anime/
│   ├── list/
│   ├── sync/
│   ├── update/
│   └── environments/
│       └── local.yml
├── docs/
├── watchlist.json               # gitignored
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
| `cmd/server/main.go` | `main.go`                                    | HTTP server entry point · port binding · router wiring                          |
| `auth/`              | `pkce, callback, browser, exchange, refresh` | Full OAuth2 PKCE flow · public package                                          |
| `token/`             | `token.go`                                   | Token struct · Save/Load to disk · expiry check (5-min buffer) · public package |
| `internal/session`   | `session.go`                                 | Token lifecycle orchestration — loads, refreshes, or re-authenticates           |
| `internal/config`    | `constants.go`                               | All global constants — API URLs, port, PKCE method, concurrency cap             |
| `internal/mal`       | `types.go, client.go`                        | MAL API client · offset pagination · typed response structs                     |
| `internal/diff`      | `types.go, loader.go, engine.go`             | Format detection · watchlist parsing · status diff computation                  |
| `internal/updater`   | `patch.go, batch.go`                         | Single-entry PATCH · concurrent batch runner · semaphore + WaitGroup            |
| `internal/server`    | `router, middleware, handlers`               | Chi router · JWT middleware · all HTTP handlers                                 |

### Why `auth/` and `token/` are public packages

Both packages are outside `internal/` so they can be imported by `media-shelf` (Project 2) as an external module dependency. `internal/` enforces a compile-time boundary — packages inside it are invisible to code outside the module.

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
┌──────────────────────────────────────────────────────────────────────┐
│   cmd/main.go                      cmd/server/main.go                │
│   CLI · --dry-run                  HTTP Server · :8080               │
└──────┬─────────────────────────────────────┬─────────────────────────┘
       │                                     │
       └──────────────────┬──────────────────┘
                          ▼
             ┌─────────────────────────┐
             │    internal/session     │
             │    LoadOrRefresh()      │
             └──────┬──────────────┬───┘
                    │              │
                    ▼              ▼
               ┌─────────┐   ┌──────────┐
               │  auth/  │   │  token/  │
               │  PKCE   │   │  CRUD    │
               │  OAuth2 │   │  Expiry  │
               └─────────┘   └──────────┘

  ┌────────────┐  ┌──────────────┐  ┌─────────────┐  ┌─────────────────┐
  │  internal  │  │   internal   │  │  internal   │  │    internal     │
  │  /config   │  │   /diff      │  │  /mal       │  │    /server      │
  │            │  │              │  │             │  │                 │
  │ Constants  │  │ Loader       │  │ API Client  │  │ Router          │
  │ Endpoints  │  │ Engine       │  │ Pagination  │  │ JWT Middleware  │
  └────────────┘  └──────┬───────┘  └──────┬──────┘  │ Handlers        │
                         │                 │         └────────┬────────┘
                         ▼                 ▼                  │
                  ┌─────────────┐   ┌─────────────┐           │
                  │  internal   │   │  External   │           │
                  │  /updater   │   │  MAL APIs   │◄──────────┘
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
│  fields: list_status, num_episodes, media_type       │
│  Paginated (limit=1000, offset-based)                │
│  → []mal.ListEntry                                   │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  4 · Load Watchlist                                  │
│                                                      │
│  CLI:  diff.LoadWatchlist("watchlist.json")          │
│        First byte == '[' ──► loadFlatArray()         │
│        First byte == '{' ──► loadCategorized()       │
│                                                      │
│  HTTP: watchlist in POST /sync request body          │
│                                                      │
│  Both paths → []diff.WatchlistEntry                  │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  5 · Compare (Diff Engine)                           │
│                                                      │
│  Build map[animeID]mal.ListEntry from MAL data       │
│  For each local entry:                               │
│    · Status mismatch?        → queue update          │
│    · Status == completed &&  → set episodes to       │
│      mal.NumEpisodes > 0       series total          │
│                                                      │
│  → []diff.Update  (N entries requiring change)       │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
              ┌────────┴────────┐
              │  updates == 0?  │
              └────┬────────┬───┘
                  YES      NO
                   │        │
                   ▼        ▼
           ┌──────────┐  ┌─────────────────┐
           │ Already  │  │  dry_run set?   │
           │ in sync  │  └────┬────────┬───┘
           │ Exit     │      YES      NO
           └──────────┘       │        │
                              ▼        ▼
                    ┌─────────────┐ ┌──────────────────────────────────┐
                    │ Return/print│ │  7 · Batch PATCH (Concurrent)    │
                    │ planned     │ │                                  │
                    │ updates     │ │  ApplyUpdates(updates, token)    │
                    │             │ │  Goroutine per update            │
                    └─────────────┘ │  Buffered channel semaphore (3)  │
                                    │  sync.WaitGroup + sync.Mutex     │
                                    │                                  │
                                    │  PATCH /anime/{id}/my_list_status│
                                    └──────────────┬───────────────────┘
                                                   │
                                                   ▼
                                    ┌──────────────────────────────────┐
                                    │  Done                            │
                                    │  N succeeded · M failed          │
                                    └──────────────────────────────────┘
```

---

## HTTP API

All protected routes require `Authorization: Bearer <token>` header.
Token is issued by `POST /auth/token` — valid for 24 hours, signed with `HS256`.

### POST /auth/token

No auth required. Issues a signed JWT.

```json
// response
{
  "token": "eyJhbGci...",
  "expires_in": 86400
}
```

### POST /sync

Diffs the supplied watchlist against MAL and applies updates.

```json
// request
{
  "watchlist": [
    { "mal_id": 1535, "watchListType": 5 },
    { "mal_id": 16498, "watchListType": 1 }
  ],
  "dry_run": false
}

// response
{
  "total": 2,
  "succeeded": 2,
  "failed": 0,
  "dry_run": false
}
```

### PATCH /anime/:id

Updates a single MAL entry. When `status` is `completed` and `episodes` is not supplied,
the total episode count is fetched from MAL and set automatically.

```json
// request
{ "status": "completed" }

// response
{ "id": 1535, "updated": true, "episodes_set": 37 }
```

### GET /anime/:id

Returns full anime details from MAL.

```json
// response (key fields)
{
  "id": 1535,
  "title": "Death Note",
  "synopsis": "...",
  "media_type": "tv",
  "status": "finished_airing",
  "num_episodes": 37,
  "mean": 8.62,
  "rank": 95,
  "popularity": 2,
  "rating": "r",
  "genres": [{ "id": 40, "name": "Psychological" }],
  "studios": [{ "id": 11, "name": "Madhouse" }],
  "start_date": "2006-10-04",
  "end_date": "2007-06-27"
}
```

### GET /anime/search

Searches MAL. Genre and status filtering is applied client-side after the MAL API response.

| Param    | Example              | Notes                              |
| -------- | -------------------- | ---------------------------------- |
| `q`      | `death note`         | Required — forwarded to MAL search |
| `genre`  | `action`             | Optional — client-side filter      |
| `status` | `finished_airing`    | Optional — client-side filter      |
| `type`   | `tv`, `movie`, `ova` | Optional — forwarded to MAL        |

### GET /list

Returns the authenticated user's MAL list with optional filters.

| Param    | Example          | Notes                  |
| -------- | ---------------- | ---------------------- |
| `status` | `watching`       | Filter by watch status |
| `type`   | `tv`             | Filter by media type   |
| `score`  | `8`              | Minimum score filter   |
| `sort`   | `title`, `score` | Sort order             |

```json
// response
{ "total": 12, "data": [...] }
```

### JWT Middleware

All routes except `POST /auth/token` are protected. The middleware:

1. Reads `Authorization: Bearer <token>` header
2. Validates signature using `JWT_SECRET`
3. Checks expiry
4. Attaches `jwt.MapClaims` to request context via `context.WithValue`
5. Returns `401` on any failure — never reaches the handler

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

| Variable           | Purpose                          |
| ------------------ | -------------------------------- |
| `MAL_CLIENT_ID`    | MAL API client ID                |
| `MAL_REDIRECT_URI` | OAuth2 callback URL              |
| `JWT_SECRET`       | Signing secret for JWT tokens    |
| `SERVER_PORT`      | HTTP server port (default: 8080) |

---

## Critical Implementation Notes

**MAL forces plain PKCE**
Setting `code_challenge_method=S256` returns `400 invalid_grant`. `PKCEMethod` is fixed to `plain` — challenge equals verifier.

**GET vs PATCH field name asymmetry**

- GET response → `num_episodes_watched`
- PATCH body → `num_watched_episodes`

**Episode auto-fill on completed**
`PATCH /anime/:id` with `status: completed` fetches `num_episodes` from MAL and sets it automatically. Same rule applies in the diff engine for CLI sync.

**Token expiry buffer**
`token.IsExpired()` returns `true` 5 minutes before actual expiry.

**Token file permissions**
`token.json` written with `0600` (owner read/write only).

**Semaphore via buffered channel**

```go
sem := make(chan struct{}, MALUpdateConcurrency) // capacity = 3
sem <- struct{}{}  // acquire
<-sem              // release
```

**Circular import resolution**
`auth/` → imports → `token/`. `internal/session/` imports both. No other package imports `session`. The dependency graph is acyclic.

**Variable shadowing — `token` package vs local variable**
Local variables holding a `token.Token` value use `tok` — not `token` — to avoid shadowing the package name and silently breaking `token.Save()` / `token.Load()` calls.

**Route order in chi**
Static routes must be registered before parametric routes:

```go
r.Get("/anime/search", h.SearchAnime)  // registered first
r.Get("/anime/{id}", h.GetAnime)       // registered second
```

Reversed order causes `"search"` to be captured as `{id}`.

**`w.Header().Set()` must precede `w.WriteHeader()`**
Headers are flushed to the client on `WriteHeader`. Any `Header().Set()` call after that is silently ignored.

**Context key typing**
`type contextKey string` prevents collisions with other packages using the same string key in `context.WithValue`. `contextKey("claims") != string("claims")` at the type level.

---

_MAL Updater · Technical Documentation · March 2026_
