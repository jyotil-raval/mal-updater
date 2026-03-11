# MAL API v2 Reference

> Internal reference for the `mal-updater` project.
> Only the endpoints this tool actually uses are documented here.
> Full official docs: https://myanimelist.net/apiconfig/references/api/v2

---

## Base URLs

| Purpose      | URL                                           |
| ------------ | --------------------------------------------- |
| API requests | `https://api.myanimelist.net/v2`              |
| OAuth2 auth  | `https://myanimelist.net/v1/oauth2/authorize` |
| OAuth2 token | `https://myanimelist.net/v1/oauth2/token`     |

---

## Authentication

All API requests require a Bearer token in the Authorization header:

```
Authorization: Bearer <access_token>
```

The token is obtained via the OAuth2 + PKCE flow. See [OAUTH_FLOW.md](./OAUTH_FLOW.md) for the
full step-by-step.

**Token lifetime:** ~1 hour (3600 seconds).
**Refresh token:** Long-lived. Use it to get a new access token without re-authenticating.

---

## Endpoints Used by This Tool

---

### 1. Get User's Anime List

Fetches the authenticated user's full anime list with status fields.

```
GET /users/@me/animelist
```

**Query Parameters:**

| Parameter | Value         | Description                          |
| --------- | ------------- | ------------------------------------ |
| `fields`  | `list_status` | Include watch status in each entry   |
| `limit`   | `1000`        | Max entries per page (MAL max: 1000) |
| `offset`  | `0` (default) | Pagination offset                    |

**Full request example:**

```
GET https://api.myanimelist.net/v2/users/@me/animelist?fields=list_status&limit=1000
Authorization: Bearer <access_token>
```

**Response shape:**

```json
{
  "data": [
    {
      "node": {
        "id": 1535,
        "title": "Death Note"
      },
      "list_status": {
        "status": "completed",
        "score": 9,
        "num_episodes_watched": 37,
        "is_rewatching": false,
        "updated_at": "2024-01-15T10:00:00+00:00"
      }
    }
  ],
  "paging": {
    "next": "https://api.myanimelist.net/v2/users/@me/animelist?offset=1000&..."
  }
}
```

**Key fields used by this tool:**

| Field                              | Type   | Description                     |
| ---------------------------------- | ------ | ------------------------------- |
| `node.id`                          | int    | MAL anime ID                    |
| `node.title`                       | string | Anime title                     |
| `list_status.status`               | string | Watch status (see values below) |
| `list_status.score`                | int    | User score (0–10, 0 = unscored) |
| `list_status.num_episodes_watched` | int    | Episodes watched                |

**Status values (GET response):**

| Value           | Meaning            |
| --------------- | ------------------ |
| `watching`      | Currently watching |
| `completed`     | Finished           |
| `on_hold`       | Paused             |
| `dropped`       | Dropped            |
| `plan_to_watch` | In queue           |

> **Pagination note:** If your list exceeds 1000 entries, the response includes
> a `paging.next` URL. This tool currently fetches one page only (limit=1000).
> If your list exceeds 1000 entries, implement pagination in Phase 5.

---

### 2. Update Anime List Entry

Updates a single anime entry on the authenticated user's list.

```
PATCH /anime/{anime_id}/my_list_status
```

**Path parameter:**

| Parameter  | Type | Description            |
| ---------- | ---- | ---------------------- |
| `anime_id` | int  | MAL anime ID to update |

**Request headers:**

```
Authorization: Bearer <access_token>
Content-Type: application/x-www-form-urlencoded
```

**Request body (url-form-encoded):**

| Field                  | Type   | Required | Description                          |
| ---------------------- | ------ | -------- | ------------------------------------ |
| `status`               | string | No       | Watch status (see values below)      |
| `score`                | int    | No       | Score 1–10. Send `0` to remove score |
| `num_watched_episodes` | int    | No       | Number of episodes watched           |

**Status values (PATCH request body):**

| Value           | Meaning            |
| --------------- | ------------------ |
| `watching`      | Currently watching |
| `completed`     | Finished           |
| `on_hold`       | Paused             |
| `dropped`       | Dropped            |
| `plan_to_watch` | In queue           |

**Full request example:**

```
PATCH https://api.myanimelist.net/v2/anime/1535/my_list_status
Authorization: Bearer <access_token>
Content-Type: application/x-www-form-urlencoded

status=completed&score=9&num_watched_episodes=37
```

**Response shape (200 OK):**

```json
{
  "status": "completed",
  "score": 9,
  "num_episodes_watched": 37,
  "is_rewatching": false,
  "updated_at": "2024-01-15T10:00:00+00:00"
}
```

**HTTP status codes:**

| Code | Meaning                              |
| ---- | ------------------------------------ |
| 200  | Success — entry updated              |
| 400  | Bad request — invalid field value    |
| 401  | Unauthorized — token missing/expired |
| 404  | Anime ID not found on MAL            |

---

## ⚠️ Critical Field Name Difference

This trips up everyone building against the MAL v2 API:

| Context            | Field name             |
| ------------------ | ---------------------- |
| GET response       | `num_episodes_watched` |
| PATCH request body | `num_watched_episodes` |

These refer to the same data — episodes watched — but MAL uses **different key
names** depending on whether you are reading or writing. Using the wrong one
in a PATCH request will send a silently ignored field, and the episode count
will not update.

---

## Rate Limiting

MAL does not publish an official rate limit. Based on community observation:

- Stay at or below **1 request per second** to avoid throttling
- This tool uses a **semaphore with a cap of 5 concurrent PATCH requests**
  to stay well within safe limits

If you receive repeated `429 Too Many Requests` responses, reduce concurrency
in `internal/mal/client.go`.

---

## OAuth2 Token Endpoint

Used to exchange an authorization code for tokens (Phase 3 → Phase 4).

```
POST https://myanimelist.net/v1/oauth2/token
Content-Type: application/x-www-form-urlencoded
```

**Request body:**

| Field           | Value                            |
| --------------- | -------------------------------- |
| `client_id`     | Your MAL client ID               |
| `grant_type`    | `authorization_code`             |
| `code`          | Authorization code from callback |
| `redirect_uri`  | Must match registered URI        |
| `code_verifier` | The original PKCE verifier       |

**Response:**

```json
{
  "token_type": "Bearer",
  "expires_in": 3600,
  "access_token": "...",
  "refresh_token": "..."
}
```

---

## Token Refresh Endpoint

Used when `access_token` has expired (Phase 4).

```
POST https://myanimelist.net/v1/oauth2/token
Content-Type: application/x-www-form-urlencoded
```

**Request body:**

| Field           | Value              |
| --------------- | ------------------ |
| `client_id`     | Your MAL client ID |
| `grant_type`    | `refresh_token`    |
| `refresh_token` | Your refresh token |

**Response:** Same shape as the token exchange response above.

---

_Last verified: March 2026 against MAL API v2_
