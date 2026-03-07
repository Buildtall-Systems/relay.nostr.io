# REST API and Relay Usage

Complete reference for programmatic interaction with the relay.nostr.io system using `nak` and `curl`.

## Prerequisites

- [nak](https://github.com/fiatjaf/nak) (Nostr Army Knife) — for generating keypairs, signing events, and publishing
- `curl` — for HTTP API calls
- `jq` — for JSON formatting (optional)
- `base64` — for encoding NIP-98 events (standard on Linux)
- An admin nsec — required for all `/api/v1/` endpoints

## NIP-98 HTTP Authentication

All `/api/v1/` endpoints require NIP-98 authentication. This means every request must carry an `Authorization` header containing a base64-encoded, signed Nostr event of kind 27235.

### How NIP-98 Works

1. Create a kind-27235 event with:
   - `content`: empty string (`""`)
   - Tag `u`: the exact URL being requested (must match `public_base_url` + path)
   - Tag `method`: the HTTP method (GET, POST, DELETE)
2. Sign the event with an admin nsec
3. Base64-encode the JSON event
4. Send as `Authorization: Nostr <base64-encoded-event>`

### Constraints

- The event timestamp must be within **60 seconds** of the server's clock
- The `u` tag must exactly match the request URL including scheme (`https://auth.nostr.io/api/v1/pubkeys`)
- The `method` tag is case-insensitive
- The signing pubkey must be in the `admin_pubkeys` table

### Constructing the Header with nak

```bash
# Pattern: generate event, base64-encode, pass as header
NIP98=$(nak event --sec <admin-nsec> -k 27235 -c "" \
  --tag u="<full-url>" --tag method=<METHOD>)

AUTH="Nostr $(echo -n "$NIP98" | base64 -w0)"

curl -s -X <METHOD> "<full-url>" -H "Authorization: $AUTH" ...
```

Because the timestamp must be fresh, always generate the NIP-98 event immediately before the curl call. Do not reuse events across requests.

## API Endpoints

Base URL: `https://auth.nostr.io`

### List Allowed Pubkeys

```
GET /api/v1/pubkeys
```

Returns all pubkeys on the allow list.

**Example:**

```bash
NIP98=$(nak event --sec <admin-nsec> -k 27235 -c "" \
  --tag u="https://auth.nostr.io/api/v1/pubkeys" \
  --tag method=GET)

curl -s "https://auth.nostr.io/api/v1/pubkeys" \
  -H "Authorization: Nostr $(echo -n "$NIP98" | base64 -w0)" | jq .
```

**Response** (200 OK):

```json
[
  {
    "hex_pubkey": "1756c40fddc3851f7813c7dbdc53712768b8604c8569d8fc7311407cd4a41106",
    "npub": "npub1zatvgr7acwz377qncldac5m3ya5tsczvs45a3lrnz9q8e49yzyrqgrpqes",
    "note": "spindle test keypair",
    "added_by": "25e82904f0b655acf96c14defc5803d4b59550b265a864768b79fbd3a47aec19",
    "added_at": "2026-03-07T15:09:23Z"
  }
]
```

### Add an Npub to the Allow List

```
POST /api/v1/pubkeys
Content-Type: application/json
```

**Request body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `npub` | string | yes | The npub to add (bech32 encoded) |
| `note` | string | no | Human-readable description of who/what this npub is |

**Example:**

```bash
NIP98=$(nak event --sec <admin-nsec> -k 27235 -c "" \
  --tag u="https://auth.nostr.io/api/v1/pubkeys" \
  --tag method=POST)

curl -s -X POST "https://auth.nostr.io/api/v1/pubkeys" \
  -H "Authorization: Nostr $(echo -n "$NIP98" | base64 -w0)" \
  -H "Content-Type: application/json" \
  -d '{"npub":"npub1zatvgr7acwz377qncldac5m3ya5tsczvs45a3lrnz9q8e49yzyrqgrpqes","note":"test user"}' | jq .
```

**Response** (201 Created):

```json
{
  "npub": "npub1zatvgr7acwz377qncldac5m3ya5tsczvs45a3lrnz9q8e49yzyrqgrpqes",
  "note": "test user",
  "added_by": "25e82904f0b655acf96c14defc5803d4b59550b265a864768b79fbd3a47aec19"
}
```

Duplicate npubs are silently ignored (INSERT OR IGNORE).

### Remove a Pubkey from the Allow List

```
DELETE /api/v1/pubkeys/{hex}
```

The path parameter `{hex}` is the hex-encoded public key (not the npub).

**Obtaining the hex pubkey from an npub:**

```bash
nak decode npub1zatvgr7acwz377qncldac5m3ya5tsczvs45a3lrnz9q8e49yzyrqgrpqes
# Output: 1756c40fddc3851f7813c7dbdc53712768b8604c8569d8fc7311407cd4a41106
```

**Example:**

```bash
HEX="1756c40fddc3851f7813c7dbdc53712768b8604c8569d8fc7311407cd4a41106"

NIP98=$(nak event --sec <admin-nsec> -k 27235 -c "" \
  --tag u="https://auth.nostr.io/api/v1/pubkeys/${HEX}" \
  --tag method=DELETE)

curl -s -X DELETE "https://auth.nostr.io/api/v1/pubkeys/${HEX}" \
  -H "Authorization: Nostr $(echo -n "$NIP98" | base64 -w0)" | jq .
```

**Response** (200 OK):

```json
{"ok": true}
```

## Error Responses

All error responses return JSON:

```json
{"error": "description of the problem"}
```

| Status | Meaning |
|--------|---------|
| 400 | Bad request (missing npub, invalid JSON) |
| 401 | Missing or invalid NIP-98 Authorization header |
| 403 | Signing pubkey is not an admin |
| 500 | Internal server error |

Common 401 causes:
- Stale timestamp (>60s skew)
- URL mismatch in `u` tag
- Method mismatch in `method` tag
- Invalid signature
- Malformed base64 or JSON

## Publishing Events via NIP-42

Once an npub is on the allow list, publish events to the relay using nak's `--auth` flag, which handles the NIP-42 authentication handshake automatically.

### Publish a Text Note (kind 1)

```bash
nak event --sec <nsec> -c "Hello from relay.nostr.io" --auth wss://relay.nostr.io
```

### Publish with Tags

```bash
nak event --sec <nsec> -c "Tagged post" \
  --tag t=nostr --tag t=test \
  --auth wss://relay.nostr.io
```

### Publish a Specific Event Kind

```bash
# Kind 30023 (long-form content)
nak event --sec <nsec> -k 30023 \
  -c "# My Article\n\nThis is long-form content." \
  --tag d=my-article --tag title="My Article" \
  --auth wss://relay.nostr.io
```

### Query Events from the Relay

```bash
# All events by a specific author
nak req -a <hex-pubkey> wss://relay.nostr.io

# Recent kind 1 notes
nak req -k 1 --limit 10 wss://relay.nostr.io

# Events since a timestamp
nak req -k 1 --since $(date -d '1 hour ago' +%s) wss://relay.nostr.io
```

## Keypair Management with nak

### Generate a New Keypair

```bash
# Generate secret key (hex)
SECRET=$(nak key generate)

# Derive public key
PUBLIC=$(echo "$SECRET" | nak key public)

# Encode to bech32
NSEC=$(nak encode nsec "$SECRET")
NPUB=$(nak encode npub "$PUBLIC")

echo "nsec: $NSEC"
echo "npub: $NPUB"
echo "hex pubkey: $PUBLIC"
```

### Decode an Existing npub/nsec

```bash
# npub to hex pubkey
nak decode npub1...

# nsec to hex secret
nak decode nsec1...
```

## Shell Helper Function

For repeated API calls, define a helper:

```bash
nip98_auth() {
  local nsec="$1" url="$2" method="$3"
  local event=$(nak event --sec "$nsec" -k 27235 -c "" \
    --tag u="$url" --tag method="$method")
  echo "Nostr $(echo -n "$event" | base64 -w0)"
}

# Usage:
AUTH=$(nip98_auth "nsec1..." "https://auth.nostr.io/api/v1/pubkeys" "GET")
curl -s "https://auth.nostr.io/api/v1/pubkeys" -H "Authorization: $AUTH" | jq .
```
