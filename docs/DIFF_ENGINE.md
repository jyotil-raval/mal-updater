# Diff Engine

> Internal reference for the `mal-updater` project.
> Covers the comparison logic between the local watchlist and the live MAL list.

---

## Overview

The diff engine takes two inputs and produces one output:

```
Local watchlist (watchlist.json)  ─┐
                                    ├──► Compare() ──► []Update
MAL anime list (live API data)    ─┘
```

It answers one question: **which entries in the local file differ from MAL,
and what exactly needs to change?**

---

## File Structure

```
internal/diff/
├── types.go     ← Update struct — represents one pending change
└── engine.go    ← Compare() function — produces the diff
```

---

## What Triggers an Update

An entry is included in the diff output if any of these differ between
the local file and MAL:

| Field              | Local source                   | MAL source             |
| ------------------ | ------------------------------ | ---------------------- |
| `status`           | `watchListType` (int → string) | `list_status.status`   |
| `episodes_watched` | `watchedEpisode` (if present)  | `num_episodes_watched` |

Score is intentionally excluded — the local HiAnime export does not carry
score data, so we never overwrite the user's MAL score.

---

## watchListType Mapping

The local file uses integer codes. MAL uses string values.
The diff engine translates:

| watchListType | MAL status string |
| ------------- | ----------------- |
| 1             | `watching`        |
| 2             | `on_hold`         |
| 3             | `plan_to_watch`   |
| 4             | `dropped`         |
| 5             | `completed`       |

---

## Update Struct

```go
type Update struct {
    AnimeID  int
    Title    string
    Status   string // MAL status string
    Episodes int    // target episode count
}
```

Each `Update` represents exactly one PATCH call to MAL in Phase 7.

---

## Compare Logic

```
Build a map of MAL entries indexed by AnimeID
  → O(1) lookup per local entry

For each entry in local watchlist:
  Look up AnimeID in MAL map
  If not found → skip (not on MAL list yet)
  If found:
    Translate watchListType → MAL status string
    Compare status and episodes
    If either differs → add to updates
```

**Why index MAL by AnimeID?**
Iterating the MAL slice for every local entry would be O(n×m).
A map makes each lookup O(1) — linear overall regardless of list size.

---

## Edge Cases

| Scenario                         | Behaviour                                                          |
| -------------------------------- | ------------------------------------------------------------------ |
| Entry in local file, not on MAL  | Skip — not our job to add new entries                              |
| Entry on MAL, not in local file  | Skip — local file is the source of truth only for what it contains |
| Status matches, episodes differ  | Include in updates                                                 |
| Episodes match, status differs   | Include in updates                                                 |
| Both match                       | Skip — no update needed                                            |
| watchListType has no MAL mapping | Skip with a warning log                                            |

---

## Usage (from cmd/main.go)

```go
updates, err := diff.Compare(watchlist, malEntries)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("%d updates needed\n", len(updates))
```

---

_Last updated: March 2026_
