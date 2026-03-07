# Admin Setup and Web UI

How to seed administrators and use the web-based admin dashboard at `https://auth.nostr.io`.

## Admin Concepts

The system has two distinct authorization tables:

| Table | Purpose | Who belongs here |
|-------|---------|-----------------|
| `admin_pubkeys` | Can manage the allow list (add/remove npubs) | Relay operators |
| `allowed_pubkeys` | Can publish events to the relay | End users |

An admin can add npubs to the allow list but is not automatically on the allow list themselves. To both manage the relay and publish to it, a pubkey must appear in both tables.

## Seeding the First Admin

Before the system is usable, at least one admin must be seeded. There are three methods:

### Method 1: Direct SQLite Insert

Requires shell access to the server.

```bash
# Decode the npub to get the hex pubkey
HEX=$(nak decode npub1...)

# Insert into admin_pubkeys
sqlite3 /var/lib/relay.nostr.io/relay-authz.db \
  "INSERT INTO admin_pubkeys (hex_pubkey, npub) VALUES ('${HEX}', 'npub1...');"
```

Verify:

```bash
sqlite3 /var/lib/relay.nostr.io/relay-authz.db "SELECT * FROM admin_pubkeys;"
```

### Method 2: Seed File with --seed Flag

Create a TOML file (`seed-admins.toml`):

```toml
admin_npubs = [
    "npub1mkq63wkt4v94cvq869njlwpszwpmf62c84p3sdvc2ptjy04jnzjs20r4tx",
    "npub1yh5zjp8ske26e7tvzn00ckqr6j6e259jvk5xga5t08aa8fr6asvsufw456",
]
```

Run the sidecar with the `--seed` flag:

```bash
relay-authz --config /etc/relay.nostr.io/authz.toml --seed seed-admins.toml
```

The sidecar decodes each npub, inserts it into `admin_pubkeys` (using INSERT OR IGNORE to avoid duplicates), then continues normal operation. The seed file can also be passed during normal startup — seeding is idempotent.

### Method 3: During Local Development

```bash
make run
```

This builds, seeds admins from `configs/seed-admins.toml`, and starts the sidecar with the dev config.

## Adding More Admins

Once the first admin is seeded, additional admins can only be added via direct database access. There is no API endpoint for admin management — this is intentional to limit the blast radius of a compromised admin key.

```bash
sqlite3 /var/lib/relay.nostr.io/relay-authz.db \
  "INSERT INTO admin_pubkeys (hex_pubkey, npub) VALUES ('<hex>', '<npub>');"
```

To remove an admin:

```bash
sqlite3 /var/lib/relay.nostr.io/relay-authz.db \
  "DELETE FROM admin_pubkeys WHERE npub = 'npub1...';"
```

To list all admins:

```bash
sqlite3 /var/lib/relay.nostr.io/relay-authz.db \
  "SELECT npub, added_at FROM admin_pubkeys;"
```

## Web UI (auth.nostr.io)

The web dashboard provides a browser-based interface for managing the allow list. It requires a NIP-07 compatible browser extension (e.g., nos2x, Alby, nostr-keyx).

### Prerequisites

- A NIP-07 browser extension installed and configured with an admin nsec
- The admin's npub must be in the `admin_pubkeys` table

### Login Flow

1. Navigate to `https://auth.nostr.io`
2. The login page appears with a "Login with Nostr" button
3. Clicking the button triggers the following sequence:

   a. The browser sends `POST /api/auth/challenge` with the pubkey from `window.nostr.getPublicKey()`

   b. The server verifies the pubkey is in `admin_pubkeys` and returns an unsigned kind-22242 event containing a random challenge string and a 5-minute expiration

   c. The browser calls `window.nostr.signEvent()` on the challenge event, prompting the user to approve the signature in their extension

   d. The signed event is sent to `POST /api/auth/verify`

   e. The server validates the signature, checks the timestamp (must be <5 minutes old), confirms admin status, and creates a session

   f. A `relay_session` cookie is set (24-hour TTL, HttpOnly, SameSite=Lax) and the browser redirects to `/dashboard`

### Dashboard

The dashboard displays:

- A table of all npubs on the allow list, showing:
  - The npub (bech32 encoded)
  - The note/description (if any)
  - Who added it (admin hex pubkey)
  - When it was added (timestamp)
  - A delete button for each entry
- An "Add npub" form with fields for npub and optional note

### Adding an npub via the Web UI

1. Enter the npub in bech32 format (`npub1...`) in the "npub" field
2. Optionally enter a note describing who this npub belongs to
3. Submit the form
4. The table refreshes via htmx to show the new entry
5. The added npub can now publish events to the relay immediately

### Removing an npub via the Web UI

1. Click the delete button next to the npub in the table
2. The entry is removed and the table refreshes
3. The removed npub can no longer publish events to the relay

Changes take effect immediately — there is no cache between the sidecar and the relay's gRPC calls.

### Session Management

- Sessions last 24 hours from creation
- The session cookie is HttpOnly (not accessible to JavaScript)
- Logging out (`POST /api/auth/logout`) clears both the cookie and the server-side session record
- Expired sessions are cleaned up automatically
- Navigating to `/` while logged in redirects to `/dashboard`

### Troubleshooting the Web UI

**"Not authorized" on login**: The pubkey from the browser extension is not in `admin_pubkeys`. Verify with:

```bash
# Get the hex pubkey from the npub shown in your extension
nak decode npub1...

# Check if it's in the admin table
sqlite3 /var/lib/relay.nostr.io/relay-authz.db \
  "SELECT COUNT(*) FROM admin_pubkeys WHERE hex_pubkey = '<hex>';"
```

**Extension popup doesn't appear**: Ensure a NIP-07 extension is installed, enabled for the auth.nostr.io domain, and has a key configured.

**Session expired**: Simply log in again. The 24-hour TTL is not refreshed on activity — it's a fixed window from login time.

## Database Schema Reference

### admin_pubkeys

```sql
CREATE TABLE admin_pubkeys (
    hex_pubkey TEXT PRIMARY KEY,
    npub       TEXT NOT NULL UNIQUE,
    added_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### allowed_pubkeys

```sql
CREATE TABLE allowed_pubkeys (
    hex_pubkey TEXT PRIMARY KEY,
    npub       TEXT NOT NULL UNIQUE,
    added_by   TEXT NOT NULL,       -- hex pubkey of the admin who added this entry
    added_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    note       TEXT DEFAULT ''
);
```

### sessions

```sql
CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    pubkey_hex TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);
```
