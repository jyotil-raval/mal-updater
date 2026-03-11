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

| Field               | Value                                                                              |
| ------------------- | ---------------------------------------------------------------------------------- |
| Language            | Go 1.26 · darwin/arm64 (Apple Silicon)                                             |
| Module              | [github.com/jyotil-raval/mal-updater](https://github.com/jyotil-raval/mal-updater) |
| External dependency | `github.com/joho/godotenv v1.5.1`                                                  |
| Status              | Complete — 9 phases shipped                                                        |

**Purpose:** Synchronises a locally exported HiAnime watchlist with a MyAnimeList (MAL) account. Handles OAuth2 authentication, reads the local watchlist in multiple formats, computes the diff against the current MAL state, and applies required updates concurrently via the MAL REST API.

---

## Package Structure

```
mal-updater/
├── cmd/
│   └── main.go                  # Entrypoint · CLI flags · Lifecycle
├── internal/
│   ├── config/
│   │   └── constants.go         # All global constants — URLs, ports, caps
│   ├── auth/
│   │   ├── pkce.go              # PKCE pair generation (plain method)
│   │   ├── callback.go          # Temporary HTTP server for OAuth2 callback
│   │   ├── browser.go           # Cross-platform browser launch
│   │   ├── exchange.go          # Authorization code → token exchange
│   │   └── refresh.go           # Silent refresh token exchange
│   ├── store/
│   │   └── token.go             # Token struct · Save/Load · Expiry check
│   ├── mal/
│   │   ├── types.go             # ListEntry, Node, ListStatus structs
│   │   └── client.go            # MAL API client · Offset pagination
│   ├── diff/
│   │   ├── types.go             # Update, WatchlistEntry structs
│   │   ├── loader.go            # Format detection · Watchlist parsing
│   │   └── engine.go            # Status diff computation
│   └── updater/
│       ├── patch.go             # Single-entry PATCH call
│       └── batch.go             # Concurrent batch runner · Semaphore + WaitGroup
├── docs/                        # Per-package reference documentation
├── watchlist.json               # gitignored — local HiAnime export
├── watchlist.example.json
├── .env                         # gitignored — MAL_CLIENT_ID, MAL_REDIRECT_URI
└── .env.example
```

---

## Architecture

### Package Responsibilities

| Package            | Key Files                                    | Responsibility                                                       |
| ------------------ | -------------------------------------------- | -------------------------------------------------------------------- |
| `cmd/main.go`      | `main.go`                                    | Entrypoint · CLI flags · Token lifecycle · Orchestration             |
| `internal/config`  | `constants.go`                               | All global constants — API URLs, port, PKCE method, concurrency cap  |
| `internal/auth`    | `pkce, callback, browser, exchange, refresh` | Full OAuth2 PKCE flow + silent token refresh                         |
| `internal/store`   | `token.go`                                   | Token struct · Save/Load to disk · Expiry check (5-min buffer)       |
| `internal/mal`     | `types.go, client.go`                        | MAL API client · Offset pagination · Typed response structs          |
| `internal/diff`    | `types.go, loader.go, engine.go`             | Format detection · Watchlist parsing · Status diff computation       |
| `internal/updater` | `patch.go, batch.go`                         | Single-entry PATCH · Concurrent batch runner · Semaphore + WaitGroup |

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  External / File System                                                      │
│                                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌───────────────┐                       │
│  │   .env      │  │ token.json  │  │watchlist.json │                       │
│  │ Client ID   │  │ Access +    │  │ Local anime   │                       │
│  │ Redirect URI│  │ Refresh     │  │ list          │                       │
│  └──────┬──────┘  └──────┬──────┘  └───────┬───────┘                       │
└─────────┼────────────────┼─────────────────┼─────────────────────────────┘
          │                │                 │
          ▼                ▼                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          cmd/main.go                                         │
│              Entry point · CLI flags (--dry-run) · Lifecycle                 │
└──────┬──────────────┬──────────────┬───────────────┬────────────────────────┘
       │              │              │               │
       ▼              ▼              ▼               ▼
┌────────────┐ ┌────────────┐ ┌──────────────┐ ┌──────────────┐
│  internal  │ │  internal  │ │   internal   │ │   internal   │
│  /config   │ │  /store    │ │   /auth      │ │   /mal       │
│            │ │            │ │              │ │              │
│ Constants  │ │ Token CRUD │ │ PKCE         │ │ API Client   │
│ Endpoints  │ │ Expiry     │ │ Callback     │ │ Pagination   │
└────────────┘ └────────────┘ │ Exchange     │ └──────┬───────┘
                               │ Refresh      │        │
                               └──────┬───────┘        │
                                      │                 │
                    ┌─────────────────┼─────────────────┘
                    │                 │
              ┌─────▼──────┐  ┌──────▼──────┐
              │  internal  │  │  External   │
              │  /diff     │  │  MAL APIs   │
              │            │  │             │
              │ Loader     │  │ OAuth2 →    │
              │ Engine     │  │ myanimelist │
              └─────┬──────┘  │             │
                    │         │ REST API →  │
                    ▼         │ api.mal.net │
              ┌─────────────┐ └─────────────┘
              │  internal   │
              │  /updater   │
              │             │
              │ Patch       │
              │ Batch       │
              │ Semaphore   │
              └─────────────┘
```

**Connection types:**

- Solid lines `─` — direct function call / package import
- `→ File System` — read/write to `.env`, `token.json`, `watchlist.json`
- `→ External API` — HTTP to MAL OAuth2 server and MAL REST API

---

## Data Flow

### Flow Diagram

```
┌─────────────────────────────────────────────────────┐
│  1 · CLI Start                                       │
│  go run cmd/main.go [--dry-run]                      │
│  flag.Parse() · godotenv.Load() · validate env vars  │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│  2 · Token Lifecycle                                 │
│                                                      │
│  store.Load(token.json)                              │
│    ├── Valid?      ──────────────────► Use directly  │
│    ├── Expired?    ──► RefreshToken() ──► Save ──►   │
│    └── Not found? ──► Full Auth Flow ──► Save ──►    │
│                                                      │
│  Full Auth: PKCE gen → Browser → Callback server     │
│             → Code exchange → token.json             │
└──────────────────────┬──────────────────────────────┘
                       │ access_token
                       ▼
┌─────────────────────────────────────────────────────┐
│  3 · Fetch MAL Anime List                            │
│                                                      │
│  GET /users/@me/animelist                            │
│  fields: list_status, num_episodes                   │
│  Paginated (limit=1000, offset-based)                │
│  → []mal.ListEntry  (currently 85 entries)           │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│  4 · Load Local Watchlist                            │
│                                                      │
│  diff.LoadWatchlist("watchlist.json")                │
│                                                      │
│  First byte == '['  ──► loadFlatArray()              │
│  First byte == '{'  ──► loadCategorized()            │
│                                                      │
│  Both paths → []diff.WatchlistEntry (869 entries)   │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│  5 · Compare (Diff Engine)                           │
│                                                      │
│  Build map[animeID]mal.ListEntry from MAL data       │
│  For each local entry:                               │
│    · Status mismatch?        → queue update          │
│    · Status == completed &&  → set episodes to       │
│      mal.NumEpisodes > 0       series total          │
│                                                      │
│  → []diff.Update  (N entries requiring change)       │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
              ┌────────┴────────┐
              │  updates == 0?  │
              └────┬──────┬─────┘
                  YES     NO
                   │      │
                   ▼      ▼
           ┌──────────┐  ┌─────────────────┐
           │ Already  │  │  --dry-run set? │
           │ in sync  │  └────┬──────┬─────┘
           │ Exit     │      YES      NO
           └──────────┘       │        │
                              ▼        ▼
                    ┌─────────────┐ ┌──────────────────────────────────┐
                    │ Print each  │ │  7 · Batch PATCH (Concurrent)    │
                    │ planned     │ │                                  │
                    │ update      │ │  ApplyUpdates(updates, token)    │
                    │ Exit        │ │  Goroutine per update            │
                    └─────────────┘ │  Buffered channel semaphore (3) │
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

**1 · CLI Start**
`flag.Parse()` reads `--dry-run` (default: `false`). `godotenv.Load()` populates `MAL_CLIENT_ID` and `MAL_REDIRECT_URI` from `.env`. Both are validated before proceeding.

**2 · Token Lifecycle**
`store.Load()` reads `token.json`. Three paths:

- Token valid → use directly, skip all auth
- Token expired → `auth.RefreshToken()` silently exchanges the refresh token with MAL; new token saved to disk
- No token / refresh fails → full browser-based PKCE flow; resulting token saved to disk

**3 · Fetch MAL Anime List**
`mal.GetAnimeList()` pages through `/users/@me/animelist` using offset-based pagination with `limit=1000`. Requests `fields=list_status,num_episodes`. All pages are accumulated into a single `[]ListEntry` slice.

**4 · Load Local Watchlist**
`diff.LoadWatchlist()` reads `watchlist.json` and inspects the first non-whitespace byte to detect format:

- `[` → flat array (new HiAnime export format, contains `mal_id` and `watchListType` fields)
- `{` → categorised object (original HiAnime export format, keyed by status category)

Both parsers produce identical `[]WatchlistEntry` output.

**5 · Compare (Diff Engine)**
`diff.Compare()` builds a `map[int]mal.ListEntry` from the MAL data keyed by anime ID. It iterates the local watchlist and queues an update when:

- The local status differs from the MAL status
- The title is `completed` and `mal.NumEpisodes > 0` — in this case episodes are auto-filled to the series total (HiAnime exports carry no episode data)

**6 · Decision Branch**

- `len(updates) == 0` → print "Already in sync" and exit
- `*dryRun == true` → print each planned update with ID, title, target status and episode count, then exit
- Otherwise → proceed to batch updater

**7 · Concurrent Batch PATCH**
`updater.ApplyUpdates()` launches one goroutine per update. A buffered channel of capacity `MALUpdateConcurrency` (3) acts as a counting semaphore. `sync.WaitGroup` coordinates completion. Errors are collected under a `sync.Mutex`. Final success/failure counts are printed.

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

| Endpoint                     | Method | Purpose                                             |
| ---------------------------- | ------ | --------------------------------------------------- |
| `/users/@me/animelist`       | GET    | Fetch full anime list with status + episode data    |
| `/anime/{id}/my_list_status` | PATCH  | Update a single entry's status and/or episode count |

---

## Configuration Reference

All constants live in `internal/config/constants.go`. No magic numbers exist elsewhere in the codebase.

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

---

## Critical Implementation Notes

**MAL forces plain PKCE**
Setting `code_challenge_method=S256` returns `400 invalid_grant: Failed to verify code_verifier`. The `PKCEMethod` constant is fixed to `plain` — challenge equals verifier.

**GET vs PATCH field name asymmetry**
The MAL API uses different field names for reading and writing episode counts:

- GET response → `num_episodes_watched`
- PATCH body → `num_watched_episodes`

**Episode count logic**
Episodes are never read from `watchlist.json` — HiAnime exports carry no episode data. For `completed` titles, MAL's own `num_episodes` value is used as the episode target.

**Token expiry buffer**
`store.IsExpired()` returns `true` 5 minutes before actual expiry to avoid race conditions between token validation and use.

**Token file permissions**
`token.json` is written with permission `0600` (owner read/write only) to protect the access and refresh tokens at rest.

**Semaphore via buffered channel**

```go
sem := make(chan struct{}, MALUpdateConcurrency) // capacity = 3
sem <- struct{}{}  // acquire — blocks if 3 goroutines already running
// ... HTTP call ...
<-sem              // release
```

This bounds concurrent outbound PATCH requests without a third-party library.

---

_MAL Updater · Technical Documentation · March 2026_
