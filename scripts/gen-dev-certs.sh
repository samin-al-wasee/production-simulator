#!/bin/sh
# Generate a local certificate authority and a server certificate for
# localhost (SAN: localhost, 127.0.0.1) into environments/local/security/certs.
# Local development only: the CA is self-signed and must not be trusted
# outside this machine. Existing files are kept; delete them to rotate.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$ROOT/environments/local/security/certs"
mkdir -p "$DIR"
umask 077

if [ -f "$DIR/server.crt" ] && [ -f "$DIR/server.key" ]; then
  echo "certificates already exist in $DIR (delete them to rotate)"
  exit 0
fi

openssl req -x509 -newkey rsa:2048 -nodes -days 30 -subj "/CN=ForgeLab Local Dev CA" \
  -keyout "$DIR/ca.key" -out "$DIR/ca.crt" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -subj "/CN=localhost" \
  -keyout "$DIR/server.key" -out "$DIR/server.csr" >/dev/null 2>&1
printf 'subjectAltName=DNS:localhost,IP:127.0.0.1\nbasicConstraints=CA:FALSE\nkeyUsage=digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\n' > "$DIR/san.ext"
openssl x509 -req -in "$DIR/server.csr" -CA "$DIR/ca.crt" -CAkey "$DIR/ca.key" -CAcreateserial \
  -days 30 -extfile "$DIR/san.ext" -out "$DIR/server.crt" >/dev/null 2>&1
rm -f "$DIR/server.csr" "$DIR/san.ext" "$DIR/ca.srl"
# The private keys stay mode 600; the nginx master process reads them as root
# before dropping privileges.
chmod 644 "$DIR/server.crt" "$DIR/ca.crt"
echo "created certificates in $DIR (valid 30 days; trust ca.crt only for local testing)"
