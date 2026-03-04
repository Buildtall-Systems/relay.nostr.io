# relay.nostr.io Status

## 2026-03-04

- Implemented full project across 7 phases (init → deployment)
- Single Go binary `relay-authz` serving gRPC (EventAdmit) + HTTP (admin webapp)
- SQLite-backed pubkey authorization replacing static TOML allowlist
- NIP-07 browser auth flow for admin login
- templ/htmx/Tailwind admin dashboard with pubkey CRUD
- Cobra CLI with --config and --seed flags, graceful shutdown
- Nix flake with Go 1.25, templ, tailwindcss v4, protoc, grpcurl
- Deployment artifacts: systemd services, NixOS module, deploy/setup scripts
