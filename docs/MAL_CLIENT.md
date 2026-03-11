# MAL API Client

> Internal reference for the `mal-updater` project.
> Covers the MAL anime list API endpoint and response shape.

---

## API Endpoint

```
GET https://api.myanimelist.net/v2/users/@me/animelist
  ?fields=list_status
  &limit=1000
  &offset=0
```

**Headers:**

```
Authorization: Bearer YOUR_ACCESS_TOKEN
```

**Query Parameters:**

| Parameter | Value         | Notes                                       |
| --------- | ------------- | ------------------------------------------- |
| `fields`  | `list_status` | Without this, MAL omits status/episode data |
| `limit`   | `1000`        | Max entries per page (MAL's ceiling)        |
| `offset`  | `0, 1000...`  | Increments by limit on each paginated page  |

> `@me` is a MAL shorthand — resolves to the authenticated user automatically.
> No user ID lookup needed.

---

## Response Shape

```json
{
  "data": [
    {
      "node": {
        "id": 1,
        "title": "Cowboy Bebop"
      },
      "list_status": {
        "status": "completed",
        "num_episodes_watched": 26,
        "score": 9
      }
    }
  ],
  "paging": {
    "next": "https://api.myanimelist.net/v2/users/@me/animelist?offset=1000&..."
  }
}
```

**Key fields:**

| Field                                     | Type   | Notes                                      |
| ----------------------------------------- | ------ | ------------------------------------------ |
| `data[].node.id`                          | int    | MAL anime ID — used as the diff engine key |
| `data[].node.title`                       | string | Human-readable name for logging            |
| `data[].list_status.status`               | string | `watching`, `completed`, `on_hold`, etc.   |
| `data[].list_status.num_episodes_watched` | int    | Current watch progress                     |
| `data[].list_status.score`                | int    | User score 0–10 (0 = not rated)            |
| `paging.next`                             | string | URL of next page. Absent when done.        |

---

## Field Naming Inconsistency

> ⚠️ MAL uses different names for the same concept depending on the operation:

| Operation | Field name             |
| --------- | ---------------------- |
| GET       | `num_episodes_watched` |
| PATCH     | `num_watched_episodes` |

The diff engine handles this translation — the client only deals with the GET field name.

---

## Status Values

| MAL status value | Meaning          |
| ---------------- | ---------------- |
| `watching`       | Currently airing |
| `completed`      | Finished         |
| `on_hold`        | Paused           |
| `dropped`        | Abandoned        |
| `plan_to_watch`  | Queue            |

---

_Last updated: March 2026_
