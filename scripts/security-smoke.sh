#!/bin/sh
# Verify the security overlay's defenses against a running stack
# (make up-secure): TLS, redirect, headers, rate limiting, blocked probing
# paths, and secret handling. All requests target localhost only.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
HTTP="http://localhost:${PROXY_HTTP_PORT:-8080}"
HTTPS="https://localhost:${PROXY_HTTPS_PORT:-8443}"
CA="$ROOT/environments/local/security/certs/ca.crt"
fail() { echo "FAIL: $1" >&2; exit 1; }
ok() { echo "ok: $1"; }

code=$(curl -s -o /dev/null -w '%{http_code}' "$HTTP/healthz")
[ "$code" = "301" ] || fail "plain HTTP should redirect (got $code)"
loc=$(curl -sI "$HTTP/healthz" | tr -d '\r' | awk 'tolower($1)=="location:" {print $2}')
case "$loc" in https://*) ;; *) fail "redirect target is not https: $loc" ;; esac
ok "http redirects to https"

code=$(curl -s --cacert "$CA" -o /dev/null -w '%{http_code}' "$HTTPS/healthz")
[ "$code" = "200" ] || fail "https /healthz should succeed with the local CA (got $code)"
ok "tls handshake verifies against the local CA"

if curl -s -o /dev/null "$HTTPS/healthz" 2>/dev/null; then fail "an untrusted client must reject the certificate"; fi
ok "certificate is not trusted without the local CA"

if curl -s -o /dev/null --cacert "$CA" --tlsv1.1 --tls-max 1.1 "$HTTPS/healthz" 2>/dev/null; then fail "TLS 1.1 must be rejected"; fi
ok "tls 1.1 rejected"

headers=$(curl -sI --cacert "$CA" "$HTTPS/healthz" | tr -d '\r')
for h in strict-transport-security x-content-type-options x-frame-options referrer-policy content-security-policy; do
  echo "$headers" | grep -qi "^$h:" || fail "missing header $h"
done
ok "security headers present"
echo "$headers" | grep -i '^server:' | grep -Eq '[0-9]+\.[0-9]+' && fail "server header leaks a version"
ok "server version not disclosed"

for path in /.env /.git/config /metrics; do
  code=$(curl -s --cacert "$CA" -o /dev/null -w '%{http_code}' "$HTTPS$path")
  [ "$code" = "404" ] || fail "$path should be 404 through the proxy (got $code)"
done
ok "dotfiles and /metrics are not served"

limited=0
for i in $(seq 1 80); do
  code=$(curl -s --cacert "$CA" -o /dev/null -w '%{http_code}' "$HTTPS/version")
  [ "$code" = "429" ] && limited=$((limited + 1))
done
[ "$limited" -gt 0 ] || fail "rate limiting never returned 429 in 80 rapid requests"
ok "rate limiting returned 429 for $limited of 80 rapid requests"

env=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' forgelab-db)
secret=$(cat "$ROOT/environments/local/security/secrets/db_password")
echo "$env" | grep -q "POSTGRES_PASSWORD=" && fail "database password is exposed as an environment value"
echo "$env" | grep -qF "$secret" && fail "database password value appears in the container environment"
ok "database password is not in the container environment"

perm=$(stat -c '%a' "$ROOT/environments/local/security/secrets/db_password")
[ "$perm" = "600" ] || fail "secret file mode is $perm, want 600"
ok "secret file is mode 600"

echo "security smoke test passed"
