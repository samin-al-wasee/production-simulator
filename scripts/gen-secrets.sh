#!/bin/sh
# Generate random local secrets into environments/local/security/secrets
# (git-ignored, mode 600 files). Existing secrets are kept.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$ROOT/environments/local/security/secrets"
mkdir -p "$DIR"
chmod 700 "$DIR"
umask 077

if [ -f "$DIR/db_password" ]; then
  echo "secrets already exist in $DIR (delete them to rotate)"
  exit 0
fi
openssl rand -hex 24 | tr -d '\n' > "$DIR/db_password"
chmod 600 "$DIR/db_password"
echo "created secrets in $DIR"
