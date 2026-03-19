#!/bin/sh
# Gaia dev entrypoint — auto-init, auto-unlock, and start in unsafe mode.
# NOT for production use.
set -e

PASS="${GAIA_DEV_PASSPHRASE:-devpassphrase}"
DIR="${GAIA_DEV_DIR:-/tmp/gaia-dev}"

mkdir -p "$DIR"

cat >&2 <<'EOF'
╔══════════════════════════════════════════════════════════════╗
║  ⚠  Gaia DEV IMAGE — UNSAFE MODE — NOT FOR PRODUCTION  ⚠   ║
║                                                              ║
║  Passphrase strength is NOT enforced.                        ║
║  Do NOT store real secrets here.                             ║
╚══════════════════════════════════════════════════════════════╝
EOF

# Auto-init if database doesn't exist yet
if [ ! -f "$DIR/gaia.db" ]; then
    echo "Initializing Gaia dev database in $DIR ..." >&2
    GAIA_PASSPHRASE="$PASS" gaia init \
        --unsafe \
        --db-file "$DIR/gaia.db" \
        --auto-certs \
        --skip-certs
    echo "Initialization complete." >&2
fi

echo "Starting Gaia dev daemon (passphrase: $PASS)..." >&2

exec gaia start --unsafe --dir "$DIR"
