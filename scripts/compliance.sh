#!/bin/sh
# Check Docker Compose files and rendered Kubernetes manifests against the
# hardening controls in core/internal/compliance, and scan for secrets.
# Kubernetes checks need kubectl (for `kubectl kustomize`) and are skipped
# with a notice when it is missing.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/core"
FL="go run ./cmd/forgelab"
status=0

echo "== secrets"
$FL security scan-secrets .. || status=1

echo "== docker compose"
$FL security compliance "$ROOT"/environments/local/compose*.yaml || status=1

if command -v kubectl >/dev/null 2>&1; then
  for target in base strategies/canary strategies/blue-green; do
    echo "== kubernetes: $target"
    kubectl kustomize "$ROOT/environments/cloud/kubernetes/$target" | $FL security compliance - || status=1
  done
else
  echo "== kubernetes: skipped (kubectl not found)"
fi
exit $status
