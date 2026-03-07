# Build and Deploy relay.nostr.io

Complete walkthrough for deploying the relay.nostr.io system from scratch on an Ubuntu server.

## Prerequisites

### Build machine

- Go 1.25+
- `git`
- `protoc` (protocol buffers compiler) with Go plugins
- `templ` (Go HTML templating)
- `tailwindcss` CLI
- If using Nix: all of the above are provided by the project's `flake.nix`

### Target server

- Ubuntu 22.04+ (or any systemd-based Linux)
- `nginx`
- `certbot` with nginx plugin
- `sqlite3` (for manual DB inspection)
- Rust toolchain (to build nostr-rs-relay) or a pre-built binary

## Step 1: Clone and Build relay-authz

```bash
git clone https://github.com/Buildtall-Systems/relay.nostr.io.git
cd relay.nostr.io
```

### With Nix (recommended)

```bash
make build
```

This runs `templ generate`, `tailwindcss` minification, and `go build` with `CGO_ENABLED=0` inside a Nix develop shell. Output: `bin/relay-authz`.

### Without Nix

Install prerequisites manually, then:

```bash
# Generate templ Go files
templ generate

# Generate Tailwind CSS (requires the buildtall theme CSS concatenated first)
tailwindcss -i static/css/input.css -o static/css/output.css --minify

# Build the binary (pure Go, no CGO)
CGO_ENABLED=0 go build \
  -ldflags "-X 'main.version=$(git describe --tags --always)' \
            -X 'main.commit=$(git rev-parse --short HEAD)' \
            -X 'main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
  -o bin/relay-authz ./cmd/relay-authz
```

## Step 2: Build or Obtain nostr-rs-relay

```bash
git clone https://sr.ht/~gheartsfield/nostr-rs-relay
cd nostr-rs-relay
cargo build --release
# Binary at target/release/nostr-rs-relay
```

## Step 3: Prepare the Server

### Create service user and directories

```bash
sudo useradd -r -s /usr/sbin/nologin nostr
sudo mkdir -p /var/lib/relay.nostr.io/static/{css,js}
sudo mkdir -p /etc/relay.nostr.io
sudo chown -R nostr:nostr /var/lib/relay.nostr.io
```

### Install binaries

```bash
sudo cp bin/relay-authz /usr/local/bin/relay-authz
sudo cp /path/to/nostr-rs-relay /usr/local/bin/nostr-rs-relay
sudo chmod +x /usr/local/bin/relay-authz /usr/local/bin/nostr-rs-relay
```

### Install static assets

```bash
sudo cp static/js/htmx.min.js /var/lib/relay.nostr.io/static/js/
sudo cp static/css/output.css /var/lib/relay.nostr.io/static/css/
```

## Step 4: Configuration

### relay-authz config (`/etc/relay.nostr.io/authz.toml`)

```toml
log_level = "INFO"
database_dir = "/var/lib/relay.nostr.io"

[grpc]
listen_address = "[::1]:50052"

[http]
listen_address = "127.0.0.1:8090"
public_base_url = "https://auth.nostr.io"
```

`public_base_url` is critical — NIP-98 URL verification matches against this value. It must exactly match the public-facing URL including scheme, without a trailing slash.

### nostr-rs-relay config (`/etc/relay.nostr.io/config.relay.toml`)

```toml
[info]
relay_url = "wss://relay.nostr.io/"
name = "relay.nostr.io"
description = "Authenticated Nostr relay"

[database]
data_directory = "/var/lib/relay.nostr.io"

[network]
address = "0.0.0.0"
port = 7780

[grpc]
event_admission_server = "http://[::1]:50052"
restricts_write = true

[authorization]
nip42_auth = true

[limits]
messages_per_sec = 3
max_event_bytes = 131072
max_ws_message_bytes = 131072
```

Key settings:
- `grpc.event_admission_server` must point to the relay-authz gRPC address
- `grpc.restricts_write = true` ensures all writes go through the sidecar
- `authorization.nip42_auth = true` enables NIP-42 authentication

## Step 5: nginx Reverse Proxy

### Install nginx and certbot

```bash
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
```

### relay.nostr.io (WebSocket relay)

Create `/etc/nginx/sites-available/relay.nostr.io`:

```nginx
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
```

### auth.nostr.io (admin HTTP interface)

Create `/etc/nginx/sites-available/auth.nostr.io`:

```nginx
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
```

### Enable sites and obtain TLS certificates

```bash
sudo ln -s /etc/nginx/sites-available/relay.nostr.io /etc/nginx/sites-enabled/
sudo ln -s /etc/nginx/sites-available/auth.nostr.io /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx

sudo certbot --nginx -d relay.nostr.io
sudo certbot --nginx -d auth.nostr.io
```

Certbot modifies the nginx configs in place to add TLS listeners and certificate paths. Verify with `sudo nginx -t` after certbot runs.

## Step 6: Systemd Services

### relay-authz sidecar (`/etc/systemd/system/relay-nostr-io-authz.service`)

```ini
[Unit]
Description=relay.nostr.io Authorization Sidecar (gRPC + HTTP)
After=network.target

[Service]
Type=simple
User=nostr
Group=nostr
WorkingDirectory=/var/lib/relay.nostr.io
ExecStart=/usr/local/bin/relay-authz --config /etc/relay.nostr.io/authz.toml
Restart=always
RestartSec=5

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/relay.nostr.io

[Install]
WantedBy=multi-user.target
```

### nostr-rs-relay (`/etc/systemd/system/relay.nostr.io.service`)

```ini
[Unit]
Description=Nostr Relay (relay.nostr.io)
After=network.target relay-nostr-io-authz.service
Requires=relay-nostr-io-authz.service

[Service]
Type=simple
User=nostr
Group=nostr
WorkingDirectory=/var/lib/relay.nostr.io
ExecStart=/usr/local/bin/nostr-rs-relay --config /etc/relay.nostr.io/config.relay.toml
Restart=always
RestartSec=5

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/relay.nostr.io

[Install]
WantedBy=multi-user.target
```

Note: The relay service has `Requires=relay-nostr-io-authz.service` — starting the relay automatically starts the sidecar first.

### Enable and start

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now relay-nostr-io-authz relay.nostr.io
```

### Verify

```bash
sudo systemctl status relay-nostr-io-authz relay.nostr.io
sudo journalctl -u relay-nostr-io-authz -f   # sidecar logs
sudo journalctl -u relay.nostr.io -f          # relay logs
```

## Step 7: Seed Admins

At least one admin npub must be seeded before the system is usable. See [admin-and-web-ui.md](admin-and-web-ui.md) for details.

Quick method using sqlite3:

```bash
# Get hex pubkey from npub
HEX=$(nak decode npub1... | jq -r .)

sqlite3 /var/lib/relay.nostr.io/relay-authz.db \
  "INSERT INTO admin_pubkeys (hex_pubkey, npub) VALUES ('${HEX}', 'npub1...');"
```

Or use the `--seed` flag with a TOML file:

```bash
relay-authz --config /etc/relay.nostr.io/authz.toml --seed seed-admins.toml
```

Where `seed-admins.toml` contains:

```toml
admin_npubs = [
    "npub1...",
]
```

## Updating

To deploy a new version:

```bash
# Build locally
make build

# Copy binary
scp bin/relay-authz root@your-server:/usr/local/bin/relay-authz.new
ssh root@your-server "mv /usr/local/bin/relay-authz.new /usr/local/bin/relay-authz && chmod +x /usr/local/bin/relay-authz"

# Copy static assets if changed
scp static/css/output.css root@your-server:/var/lib/relay.nostr.io/static/css/
scp static/js/htmx.min.js root@your-server:/var/lib/relay.nostr.io/static/js/

# Restart
ssh root@your-server "systemctl restart relay-nostr-io-authz"
```

The relay service will also restart due to the `Requires` dependency.

## Troubleshooting

### Sidecar won't start

Check config path and permissions:

```bash
ls -la /etc/relay.nostr.io/authz.toml
ls -la /var/lib/relay.nostr.io/
sudo -u nostr /usr/local/bin/relay-authz --config /etc/relay.nostr.io/authz.toml
```

### NIP-98 API returns 401

- Verify `public_base_url` in `authz.toml` matches the URL used in the NIP-98 event's `u` tag exactly (including scheme, no trailing slash)
- Check server clock — NIP-98 events must be within 60 seconds of server time
- Confirm the signing pubkey is in `admin_pubkeys` table

### Events rejected by relay

```bash
# Check if pubkey is in allowed list
sqlite3 /var/lib/relay.nostr.io/relay-authz.db \
  "SELECT npub FROM allowed_pubkeys WHERE hex_pubkey = '<hex>';"

# Check sidecar logs for DENY reasons
journalctl -u relay-nostr-io-authz --since "5 minutes ago" | grep DENY
```

### WebSocket connection fails

Verify nginx WebSocket proxy headers are present and the relay is listening:

```bash
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: test" \
  https://relay.nostr.io/
```
