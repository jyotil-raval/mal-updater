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

## Flow Diagram

```
Your CLI                    Browser                   MAL Auth Server
    |                          |                             |
    |-- Generate PKCE pair --->|                             |
    |   verifier (secret)      |                             |
    |   challenge (hash)       |                             |
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

The CLI generates two values before opening the browser:

```
code_verifier  = BASE64URL(32 random bytes)
code_challenge = BASE64URL(SHA256(code_verifier))
```

- `code_verifier` is kept in memory — never sent until token exchange
- `code_challenge` is sent to MAL in the auth URL

**Go implementation:** `internal/auth/pkce.go` → `GeneratePKCE()`

---

### Step 2 — Open Authorization URL

The CLI constructs and prints (or opens) this URL:

```
https://myanimelist.net/v1/oauth2/authorize
  ?response_type=code
  &client_id=YOUR_CLIENT_ID
  &redirect_uri=http://localhost:8080/callback
  &code_challenge=YOUR_CODE_CHALLENGE
  &code_challenge_method=S256
```

**Parameters:**

| Parameter               | Value                            | Notes                             |
| ----------------------- | -------------------------------- | --------------------------------- |
| `response_type`         | `code`                           | Always `code` for this flow       |
| `client_id`             | Your MAL client ID               | From `.env`                       |
| `redirect_uri`          | `http://localhost:8080/callback` | Must match registered URI exactly |
| `code_challenge`        | SHA256 hash of verifier          | Base64url, no padding             |
| `code_challenge_method` | `S256`                           | Always `S256` — not `plain`       |

The user logs into MAL in their browser and approves the request.

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

MAL hashes the `code_verifier`, compares it to the `code_challenge` sent in
Step 2, and only issues tokens if they match. This proves the entity exchanging
the code is the same entity that started the flow.

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
  "expires_at": "2026-03-11T15:30:00Z"
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

---

_Last updated: March 2026_
