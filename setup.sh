#!/usr/bin/env bash
set -euo pipefail

echo "Setting up relay.nostr.io services..."

# Ensure user exists
if ! id nostr &>/dev/null; then
    useradd -r -m -d /var/lib/relay.nostr.io -s /usr/sbin/nologin nostr
    echo "Created nostr user"
fi

# Ensure directories
mkdir -p /var/lib/relay.nostr.io
chown nostr:nostr /var/lib/relay.nostr.io

# Ensure authz config exists
if [ ! -f /etc/relay.nostr.io/authz.toml ]; then
    cp /etc/relay.nostr.io/authz.toml.example /etc/relay.nostr.io/authz.toml
    echo "Created authz.toml from example — edit before starting!"
fi

# Reload systemd and enable services
systemctl daemon-reload
systemctl enable relay-nostr-io-authz.service relay.nostr.io.service

echo ""
echo "Services enabled. To start:"
echo "  systemctl start relay-nostr-io-authz"
echo "  systemctl start relay.nostr.io"
echo ""
echo "Don't forget to:"
echo "  1. Edit /etc/relay.nostr.io/authz.toml with production settings"
echo "  2. Seed admin npubs: relay-authz --config /etc/relay.nostr.io/authz.toml --seed /path/to/seed-admins.toml"
echo "  3. Configure reverse proxy: wss://relay.nostr.io → :7778, https://auth.nostr.io → :8090"
