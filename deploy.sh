#!/usr/bin/env bash
set -euo pipefail

HOST="${DEPLOY_HOST:-nostr.io}"
REMOTE_USER="${DEPLOY_USER:-root}"

echo "Building relay-authz..."
nix develop -c go build -ldflags "-X 'main.version=$(git describe --tags --always)' -X 'main.commit=$(git rev-parse --short HEAD)' -X 'main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" -o bin/relay-authz ./cmd/relay-authz

echo "Deploying to ${HOST}..."

# Binary
scp bin/relay-authz "${REMOTE_USER}@${HOST}:/usr/local/bin/relay-authz.new"
ssh "${REMOTE_USER}@${HOST}" "mv /usr/local/bin/relay-authz.new /usr/local/bin/relay-authz && chmod +x /usr/local/bin/relay-authz"

# Config
ssh "${REMOTE_USER}@${HOST}" "mkdir -p /etc/relay.nostr.io /var/lib/relay.nostr.io"
scp config.relay.toml "${REMOTE_USER}@${HOST}:/etc/relay.nostr.io/config.relay.toml"
scp configs/production.toml.example "${REMOTE_USER}@${HOST}:/etc/relay.nostr.io/authz.toml.example"

# Systemd services
scp services/relay.nostr.io.service "${REMOTE_USER}@${HOST}:/etc/systemd/system/"
scp services/relay-nostr-io-authz.service "${REMOTE_USER}@${HOST}:/etc/systemd/system/"

# Static files for the web UI
ssh "${REMOTE_USER}@${HOST}" "mkdir -p /var/lib/relay.nostr.io/static/{css,js}"
scp static/js/htmx.min.js "${REMOTE_USER}@${HOST}:/var/lib/relay.nostr.io/static/js/"
scp static/css/output.css "${REMOTE_USER}@${HOST}:/var/lib/relay.nostr.io/static/css/" 2>/dev/null || echo "Warning: output.css not found, run 'make generate-css' first"

echo "Deploy complete. Run setup.sh on the remote host to enable and start services."
