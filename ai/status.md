# relay.nostr.io Status

## 2026-03-08

- Canonical npub refactor: npub is now the sole internal representation across all layers
  - DB migration 002: npub becomes PRIMARY KEY in admin_pubkeys and allowed_pubkeys, hex_pubkey columns dropped
  - Renamed added_by/added_at → created_by/created_at in allowed_pubkeys
  - Sessions store npub instead of hex pubkey
  - Hex→npub conversion at all 4 inbound boundaries (gRPC, NIP-07, NIP-98, challenge verify)
  - Npub→hex only at go-nostr API call sites (challenge creation, signature verification)
  - API routes changed from {hex} to {npub}, hex_pubkey dropped from JSON responses
  - Admins implicitly whitelisted for relay publishing (UNION query)
- Unified UI with buildtall.systems/sayer visual style
- Replaced hardcoded Tailwind gray/purple classes with iceberg theme semantic variables (bg-bg, text-fg, bg-primary, text-danger, etc.)
- Added buildtall logo (buildtall-triangles-4ply.svg) to header matching sayer pattern
- Added AvenirNextLTPro font with @font-face declarations in custom.css
- Added sticky header with shadow, footer with GitHub/buildtall.systems links
- Updated all 4 templ files: layout, login, dashboard, pubkey_table

## 2026-03-04

- Implemented full project across 7 phases (init → deployment)
- Single Go binary `relay-authz` serving gRPC (EventAdmit) + HTTP (admin webapp)
- SQLite-backed pubkey authorization replacing static TOML allowlist
- NIP-07 browser auth flow for admin login
- templ/htmx/Tailwind admin dashboard with pubkey CRUD
- Cobra CLI with --config and --seed flags, graceful shutdown
- Nix flake with Go 1.25, templ, tailwindcss v4, protoc, grpcurl
- Deployment artifacts: systemd services, NixOS module, deploy/setup scripts
