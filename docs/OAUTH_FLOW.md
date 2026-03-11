# OAuth2 + PKCE Flow

> Internal reference for the `mal-updater` project.
> Explains the full authentication flow used to obtain a MAL access token.

---

## Why PKCE?

MAL issues you a **public client** — a client ID with no client secret.
Public clients (CLI tools, mobile apps) cannot securely store a secret,
so the standard OAuth2 flow doesn't apply.

PKCE (Proof Key for Code Exchange, RFC 7636) solves this. Instead of a
shared secret, the client generates a one-time cryptographic challenge.
The auth server verifies the challenge at token exchange — without ever
storing the secret itself.

**Result:** Secure OAuth2 for public clients, no secret required.

---

## MAL-Specific Behavior

> ⚠️ This is a known undocumented quirk in MAL's OAuth2 implementation.

Despite the OAuth2 PKCE spec recommending `S256`, MAL's token endpoint
validates using the `plain` method — the `code_verifier` is compared
directly to the stored `code_challenge` without hashing.

**What this means in practice:**

- `code_challenge_method` is set to `plain` in this implementation
- `pkce.Challenge` equals `pkce.Verifier` — no SHA256 step applied
- Sending `S256` in the auth URL causes a `400 invalid_grant` error
  with hint: `"Failed to verify code_verifier"`

This behavior is consistent across all known community MAL API clients.

---

## Flow Diagram

```
Your CLI                    Browser                   MAL Auth Server
    |                          |                             |
    |-- Generate PKCE pair --->|                             |
    |   verifier (secret)      |                             |
    |   challenge = verifier   |                             |
    |   (plain method)         |                             |
    |                          |                             |
    |-- Open auth URL -------->|                             |
    |   (with code_challenge)  |-- User logs in ----------->|
    |                          |<-- Redirect to localhost ---|
    |                          |    ?code=XXXX              |
    |<-- Capture code ---------|                             |
    |   (local HTTP server)    |                             |
    |                          |                             |
    |-- POST /token ---------------------------------------->|
    |   (code + code_verifier)                               |
    |<-- access_token + refresh_token -----------------------|
    |                          |                             |
    |-- Save tokens to disk -->|                             |
    |-- Use access_token for API calls                       |
```

---

## Step-by-Step

### Step 1 — Generate PKCE Pair

The CLI generates the verifier before opening the browser:

```
code_verifier  = BASE64URL(32 random bytes)
code_challenge = code_verifier  ← plain method, no hashing
```

- `code_verifier` is kept in memory — never sent until token exchange
- `code_challenge` is sent to MAL in the auth URL

**Go implementation:** `internal/auth/pkce.go` → `GeneratePKCE()`

---

### Step 2 — Open Authorization URL

The CLI constructs and opens this URL:

```
https://myanimelist.net/v1/oauth2/authorize
  ?response_type=code
  &client_id=YOUR_CLIENT_ID
  &redirect_uri=http://localhost:8080/callback
  &code_challenge=YOUR_CODE_CHALLENGE
  &code_challenge_method=plain
```

**Parameters:**

| Parameter               | Value                            | Notes                             |
| ----------------------- | -------------------------------- | --------------------------------- |
| `response_type`         | `code`                           | Always `code` for this flow       |
| `client_id`             | Your MAL client ID               | From `.env`                       |
| `redirect_uri`          | `http://localhost:8080/callback` | Must match registered URI exactly |
| `code_challenge`        | Same as verifier                 | plain method — no hashing         |
| `code_challenge_method` | `plain`                          | MAL does not support `S256`       |

The user logs into MAL in their browser and approves the request.

**Go implementation:** `cmd/main.go` → `url.Values` + `auth.OpenBrowser()`

---

### Step 3 — Capture the Authorization Code

After approval, MAL redirects the browser to:

```
http://localhost:8080/callback?code=AUTHORIZATION_CODE
```

The CLI runs a temporary local HTTP server on port `8080` to capture this
redirect. Once the code is extracted from the URL query parameter, the server
shuts itself down.

**Go implementation:** `internal/auth/callback.go` → `WaitForCode()`

The `code` is single-use and short-lived (expires in minutes).
Do not store it — exchange it immediately.

---

### Step 4 — Exchange Code for Tokens

The CLI makes a POST request to the MAL token endpoint:

```
POST https://myanimelist.net/v1/oauth2/token
Content-Type: application/x-www-form-urlencoded

client_id=YOUR_CLIENT_ID
&grant_type=authorization_code
&code=AUTHORIZATION_CODE
&redirect_uri=http://localhost:8080/callback
&code_verifier=YOUR_CODE_VERIFIER
```

MAL compares the `code_verifier` directly to the stored `code_challenge`
(plain method). If they match, tokens are issued.

**Response:**

```json
{
  "token_type": "Bearer",
  "expires_in": 3600,
  "access_token": "...",
  "refresh_token": "..."
}
```

**Go implementation:** `internal/auth/exchange.go` → `ExchangeCode()`

---

### Step 5 — Store Tokens

Tokens are written to `token.json` in the project root with file permission
`0600` (owner read/write only).

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "token_type": "Bearer",
  "expires_at": "2026-04-11T16:51:07Z"
}
```

**Key design decision:** `expires_at` is stored as an absolute timestamp,
not the raw `expires_in` seconds from the response. This allows correct
expiry checks regardless of when the file is loaded.

**Go implementation:** `internal/store/token.go` → `Save()` / `Load()`

> Add `token.json` to `.gitignore`. It contains your personal access credentials.

---

### Step 6 — Token Refresh (Automatic)

On startup, the CLI loads `token.json` and checks if the access token is
within 5 minutes of expiry. If so, it silently refreshes using the refresh token:

```
POST https://myanimelist.net/v1/oauth2/token
Content-Type: application/x-www-form-urlencoded

client_id=YOUR_CLIENT_ID
&grant_type=refresh_token
&refresh_token=YOUR_REFRESH_TOKEN
```

The new `access_token` and `refresh_token` are saved back to `token.json`.

**If the refresh token is also expired:** The CLI prompts for a full
re-authentication (Steps 1–5 again).

**Go implementation:** `internal/auth/refresh.go` → `RefreshToken()`

---

## Token Lifecycle Summary

```
First run:
  No token.json → Full auth flow (Steps 1–5) → Save token

Subsequent runs (token valid):
  Load token.json → Access token valid → Use directly

Subsequent runs (token near expiry):
  Load token.json → Access token expiring → Refresh → Save new token → Use

Subsequent runs (refresh token expired):
  Load token.json → Refresh fails → Full auth flow again
```

---

## Security Notes

| Risk                         | Mitigation                                          |
| ---------------------------- | --------------------------------------------------- |
| Authorization code intercept | PKCE — intercepted code is useless without verifier |
| token.json exposure          | File permission `0600` + `.gitignore`               |
| Credentials in shell history | Loaded from `.env`, never passed as CLI flags       |
| Client secret exposure       | No client secret — PKCE eliminates the need         |
| S256 vs plain                | MAL forces plain — no workaround available          |

---

_Last updated: March 2026_

---

## Token Refresh Flow

### Why Refresh?

The access token expires in 31 days. Without refresh, the user must
re-authenticate manually every month. The refresh token lasts much longer
and can silently obtain a new access token without browser interaction.

### Refresh Endpoint

```
POST https://myanimelist.net/v1/oauth2/token
Content-Type: application/x-www-form-urlencoded

client_id=YOUR_CLIENT_ID
&grant_type=refresh_token
&refresh_token=YOUR_REFRESH_TOKEN
```

Response shape is identical to the initial token exchange — same fields,
new values. The new token pair completely replaces the old one.

### Refresh Logic (in cmd/main.go)

```
Load token.json
If token is valid → use directly
If token is expired:
  Attempt RefreshToken()
  If refresh succeeds → save new token → use
  If refresh fails → full auth flow (Steps 1–5)
```

### When Does the Refresh Token Expire?

MAL does not document refresh token lifetime. In practice, refresh tokens
appear to be long-lived (months). If `RefreshToken()` returns a 400 error,
the tool falls back to full re-authentication automatically.

### Go implementation

`internal/auth/refresh.go` → `RefreshToken(clientID, refreshToken string) (store.Token, error)`
