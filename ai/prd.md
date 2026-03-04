# relay.nostr.io — Authenticated Nostr Relay with Admin Webapp

## Overview

A new Nostr relay at `relay.nostr.io` with a dynamic npub authorization system: a SQLite-backed Go sidecar that also serves an admin webapp at `auth.nostr.io` where administrators authenticate via NIP-07 browser extensions and manage the authorized pubkey list in real time.

## Problem

The existing relay at `relay.drss.io` uses a static-config Go gRPC sidecar for NIP-42 authorization. The npub allowlist lives in a TOML file — adding users requires redeploying config and restarting the service.

## Solution

**Single Go binary** (`relay-authz`) serving two interfaces:
- **gRPC** on `[::1]:50052` — same `nauthz.proto` contract, answers `EventAdmit` from nostr-rs-relay
- **HTTP** on `127.0.0.1:8090` — templ/htmx/Tailwind admin webapp at `auth.nostr.io`

**Shared nostr-rs-relay binary** — same `/usr/local/bin/nostr-rs-relay` used by relay.drss.io, with a separate config on port 7778 and data dir `/var/lib/relay.nostr.io`.

## Auth Flow (NIP-07)

1. `POST /api/auth/challenge` with `{"pubkey":"<hex>"}` — server returns unsigned kind-22242 event with random challenge tag
2. Client signs via `window.nostr.signEvent()`, sends `POST /api/auth/verify` with signed event
3. Server verifies signature + timestamp + pubkey is in `admin_pubkeys`, creates session cookie
4. Subsequent requests use session cookie, validated by `RequireAdmin` middleware

## HTTP Routes

- `GET /` — login page (redirect to /dashboard if session exists)
- `GET /dashboard` — admin dashboard [RequireAdmin]
- `POST /api/auth/challenge` — generate challenge
- `POST /api/auth/verify` — verify + create session
- `POST /api/auth/logout` — clear session
- `GET /api/pubkeys` — list allowed pubkeys [RequireAdmin, htmx partial]
- `POST /api/pubkeys` — add npub [RequireAdmin]
- `DELETE /api/pubkeys/{hex}` — remove npub [RequireAdmin]

## Port Allocation

| Service | Port | Binding |
|---------|------|---------|
| relay.nostr.io relay | 7778 | 0.0.0.0 |
| relay.nostr.io authz gRPC | 50052 | [::1] |
| relay.nostr.io authz HTTP | 8090 | 127.0.0.1 |

## Tech Stack

- Go with Cobra/Viper CLI
- modernc.org/sqlite (pure-Go, CGO_ENABLED=0)
- goose for migrations
- go-nostr for Nostr types + signature verification
- gRPC for nostr-rs-relay integration
- templ/htmx/Tailwind for admin webapp
- NixOS for deployment
