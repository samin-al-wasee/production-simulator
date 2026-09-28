#!/bin/sh
# Point the sample-web Service at one blue/green slot (instant cutover or
# rollback). Usage: scripts/k8s-bluegreen-switch.sh <blue|green>
set -eu
slot="${1:-}"
case "$slot" in
  blue|green) ;;
  *) echo "usage: $0 <blue|green>" >&2; exit 2 ;;
esac
kubectl -n forgelab rollout status "deploy/sample-web-$slot" --timeout=120s
kubectl -n forgelab patch service sample-web \
  --type=merge -p "{\"spec\":{\"selector\":{\"app\":\"sample-web\",\"slot\":\"$slot\"}}}"
echo "sample-web now routes to slot: $slot"
