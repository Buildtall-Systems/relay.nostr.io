#!/usr/bin/env bash
# setup.sh — idempotent first-time setup for relay.nostr.io on a remote host
# Run as root after deploy.sh has pushed binaries, configs, and service units.
set -euo pipefail

DATA_DIR="/var/lib/relay.nostr.io"
CONF_DIR="/etc/relay.nostr.io"
USER="nostr"
GROUP="nostr"

echo "=== relay.nostr.io setup ==="

# --- User/Group (idempotent) ---
if ! getent group "${GROUP}" >/dev/null 2>&1; then
    groupadd --system "${GROUP}"
    echo "Created group: ${GROUP}"
else
    echo "Group already exists: ${GROUP}"
fi

if ! id -u "${USER}" >/dev/null 2>&1; then
    useradd --system --gid "${GROUP}" --home-dir "${DATA_DIR}" --no-create-home --shell /usr/sbin/nologin "${USER}"
    echo "Created user: ${USER}"
else
    echo "User already exists: ${USER}"
fi

# --- Directories ---
mkdir -p "${DATA_DIR}" "${DATA_DIR}/static/css" "${DATA_DIR}/static/js"
mkdir -p "${CONF_DIR}"
chown -R "${USER}:${GROUP}" "${DATA_DIR}"
echo "Directories ready"

# --- Authz config (only if absent) ---
if [ ! -f "${CONF_DIR}/authz.toml" ]; then
    cat > "${CONF_DIR}/authz.toml" <<'TOML'
log_level = "INFO"
database_dir = "/var/lib/relay.nostr.io"

[grpc]
listen_address = "[::1]:50052"

[http]
listen_address = "127.0.0.1:8090"
public_base_url = "https://auth.nostr.io"
TOML
    echo "Created ${CONF_DIR}/authz.toml"
else
    echo "Authz config already exists, skipping"
fi

# --- Seed admins config (only if absent) ---
if [ ! -f "${CONF_DIR}/seed-admins.toml" ]; then
    cat > "${CONF_DIR}/seed-admins.toml" <<'TOML'
admin_npubs = [
    "npub1mkq63wkt4v94cvq869njlwpszwpmf62c84p3sdvc2ptjy04jnzjs20r4tx",
]
TOML
    echo "Created ${CONF_DIR}/seed-admins.toml"
else
    echo "Seed admins config already exists, skipping"
fi

# --- Seed admin pubkeys into database ---
echo "Seeding admin pubkeys..."
/usr/local/bin/relay-authz --config "${CONF_DIR}/authz.toml" --seed "${CONF_DIR}/seed-admins.toml" &
SEED_PID=$!
sleep 2
kill "${SEED_PID}" 2>/dev/null || true
wait "${SEED_PID}" 2>/dev/null || true
chown -R "${USER}:${GROUP}" "${DATA_DIR}"
echo "Admin seed complete"

# --- Nginx reverse proxy (only if absent) ---
NGINX_CONF="/etc/nginx/sites-available/relay.nostr.io"
if [ ! -f "${NGINX_CONF}" ]; then
    cat > "${NGINX_CONF}" <<'NGINX'
# relay.nostr.io — Nostr relay WebSocket proxy
server {
    listen 80;
    server_name relay.nostr.io;

    location / {
        proxy_pass http://127.0.0.1:7780;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }
}

# auth.nostr.io — Admin UI + NIP-98 REST API
server {
    listen 80;
    server_name auth.nostr.io;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
NGINX
    ln -sf "${NGINX_CONF}" /etc/nginx/sites-enabled/relay.nostr.io
    echo "Created nginx config (port 80 — run certbot after DNS is pointed)"
else
    echo "Nginx config already exists, skipping"
fi

# --- Systemd ---
systemctl daemon-reload
systemctl enable relay-nostr-io-authz.service relay.nostr.io.service
echo "Services enabled"

# --- Nginx validation and reload ---
nginx -t && systemctl reload nginx
echo "Nginx reloaded"

echo ""
echo "=== Setup complete ==="
echo ""
echo "Next steps:"
echo "  1. Point DNS:  relay.nostr.io -> this droplet"
echo "                 auth.nostr.io  -> this droplet"
echo "  2. TLS:        certbot --nginx -d relay.nostr.io -d auth.nostr.io"
echo "  3. Start:      systemctl start relay-nostr-io-authz relay.nostr.io"
echo "  4. Verify:     curl -I https://auth.nostr.io/"
