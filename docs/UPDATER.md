# Updater

> Internal reference for the `mal-updater` project.
> Covers the MAL PATCH endpoint, concurrent update execution, and rate limiting.

---

## Overview

The updater takes the `[]Update` slice from the diff engine and applies
each change to MAL via PATCH requests — concurrently, with a semaphore
capping the number of requests in-flight at any moment.

```
[]Update ──► ApplyUpdates() ──► N concurrent PatchAnime() calls ──► MAL API
                                 (capped by semaphore)
```

---

## File Structure

```
internal/updater/
├── patch.go     ← PatchAnime() — single PATCH call to MAL
└── batch.go     ← ApplyUpdates() — concurrent execution with rate limiting
```

---

## PATCH Endpoint

```
PATCH https://api.myanimelist.net/v2/anime/{anime_id}/my_list_status
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer YOUR_ACCESS_TOKEN
```

**Body parameters:**

| Field                  | Type   | Notes                                    |
| ---------------------- | ------ | ---------------------------------------- |
| `status`               | string | `watching`, `completed`, `on_hold`, etc. |
| `num_watched_episodes` | int    | Current episode count                    |

> ⚠️ Field naming inconsistency: GET returns `num_episodes_watched`.
> PATCH body requires `num_watched_episodes`. Different names, same concept.

**Success response:** `200 OK` with updated list status object.

---

## Concurrency Model

Each PATCH is independent — no update depends on the result of another.
This makes the workload **embarrassingly parallel** — ideal for goroutines.

```
ApplyUpdates(updates, token):
  Create WaitGroup
  Create semaphore (buffered channel, capacity N)

  For each update:
    wg.Add(1)
    go func():
      semaphore ← acquire    (blocks if N slots already taken)
      PatchAnime(update)
      semaphore → release
      wg.Done()

  wg.Wait()                  (blocks until all goroutines finish)
  return collected errors
```

---

## Rate Limiting via Semaphore

A semaphore controls how many goroutines run concurrently.
In Go, a buffered channel is the idiomatic semaphore:

```go
sem := make(chan struct{}, 5) // capacity 5 = max 5 concurrent requests

// Acquire — blocks if channel is full
sem <- struct{}{}

// Release — frees one slot
<-sem
```

**Why a buffered channel works as a semaphore:**

- A buffered channel of capacity N holds at most N items before blocking
- Sending to a full channel blocks the goroutine — this IS the rate limit
- Receiving from the channel frees a slot — this IS the release

No third-party library needed. This is idiomatic Go.

**Semaphore capacity:** `config.MALUpdateConcurrency = 3`
Safe for MAL's unofficial ~1 req/sec limit while still being faster than sequential.

---

## Error Collection

Multiple goroutines writing to the same variable is a data race.
A mutex protects the shared error slice:

```go
var (
    mu     sync.Mutex
    errors []error
)

// Inside goroutine — safe write
mu.Lock()
errors = append(errors, err)
mu.Unlock()
```

Only failed updates are collected. Successful updates are logged immediately.
After all goroutines finish, the caller receives the full error list.

---

## PATCH Request Shape

Unlike GET, PATCH sends data in the request body as form-encoded values:

```go
form := url.Values{}
form.Set("status", update.Status)
form.Set("num_watched_episodes", strconv.Itoa(update.Episodes))

req, err := http.NewRequest("PATCH", endpoint,
    strings.NewReader(form.Encode()))
req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
req.Header.Set("Authorization", "Bearer "+accessToken)
```

Three differences from the GET request in Phase 5:

1. Method is `PATCH` not `GET`
2. Body carries the form data — GET had no body
3. `Content-Type` header is required — tells MAL the body is form-encoded

---

## Success and Failure Handling

| Scenario           | Behaviour                                         |
| ------------------ | ------------------------------------------------- |
| `200 OK`           | Log success, continue                             |
| `400 Bad Request`  | Collect error, continue remaining updates         |
| `401 Unauthorized` | Collect error — token likely expired              |
| `404 Not Found`    | Collect error — anime ID not on user's list       |
| Network failure    | Collect error, continue remaining updates         |
| All updates fail   | Return all collected errors after WaitGroup exits |

Errors are **collected, not fatal** — one failed PATCH does not abort the rest.

---

## Usage (from cmd/main.go)

```go
errs := updater.ApplyUpdates(updates, token.AccessToken)
if len(errs) > 0 {
    for _, err := range errs {
        log.Printf("update failed: %v", err)
    }
}
```

---

## Constants Added to config/constants.go

```go
MALUpdateConcurrency = 3  // max concurrent PATCH requests
```

---

_Last updated: March 2026_
