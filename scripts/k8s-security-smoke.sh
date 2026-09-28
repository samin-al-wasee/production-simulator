#!/bin/sh
# Verify the Kubernetes environment's security controls against a running
# cluster (make k8s-up): TLS, Pod Security admission, network segmentation,
# and RBAC least privilege. Only creates short-lived pods labelled
# app=intruder in the forgelab namespace and removes them afterwards.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CA="$ROOT/environments/local/security/certs/ca.crt"
NS=forgelab
fail() { echo "FAIL: $1" >&2; exit 1; }
ok() { echo "ok: $1"; }
can() { kubectl auth can-i "$@" -n "$NS" 2>/dev/null || true; }

# --- Transport ---
[ "$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8081/version)" = "200" ] || fail "HTTP ingress not reachable on :8081"
[ "$(curl -s --cacert "$CA" -o /dev/null -w '%{http_code}' https://localhost:8444/version)" = "200" ] || fail "HTTPS ingress not reachable or certificate not verified on :8444"
ok "ingress serves HTTP on :8081 and verified HTTPS on :8444"

# --- Pod Security admission (restricted) ---
out=$(kubectl -n "$NS" run privileged-probe --image=busybox:1.36 --restart=Never \
  --overrides='{"spec":{"containers":[{"name":"privileged-probe","image":"busybox:1.36","securityContext":{"privileged":true}}]}}' \
  -- sleep 1 2>&1 || true)
echo "$out" | grep -qi "violates PodSecurity" || { kubectl -n "$NS" delete pod privileged-probe --ignore-not-found >/dev/null 2>&1; fail "a privileged pod was admitted: $out"; }
ok "privileged pod rejected by Pod Security admission"

# --- Network segmentation ---
probe() { # $1 = target host, $2 = port, $3 = pod label app value; prints rc
  kubectl -n "$NS" delete pod intruder --ignore-not-found --wait=true >/dev/null 2>&1
  kubectl -n "$NS" run intruder --image=busybox:1.36 --restart=Never --labels app="${3:-intruder}" \
    --overrides='{"spec":{"automountServiceAccountToken":false,"securityContext":{"runAsNonRoot":true,"runAsUser":65532,"seccompProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"intruder","image":"busybox:1.36","command":["sh","-c","nc -z -w 4 '"$1"' '"$2"'; echo rc=$?"],"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}' >/dev/null 2>&1
  for i in $(seq 1 40); do
    phase=$(kubectl -n "$NS" get pod intruder -o jsonpath='{.status.phase}' 2>/dev/null || true)
    [ "$phase" = "Succeeded" ] || [ "$phase" = "Failed" ] && break
    sleep 1
  done
  kubectl -n "$NS" logs intruder 2>/dev/null | tail -1
  kubectl -n "$NS" delete pod intruder --ignore-not-found --wait=false >/dev/null 2>&1
}
# Positive control: a pod carrying the allowed label reaches postgres, so a
# failure for the intruder is caused by the policy and not by DNS or timing.
res=$(probe postgres 5432 sample-web)
[ "$res" = "rc=0" ] || fail "allow-listed pod could not reach postgres ($res); the negative test would prove nothing"
ok "allow-listed pod reaches postgres (control)"
res=$(probe postgres 5432)
[ "$res" != "rc=0" ] || fail "an unlabelled pod reached postgres; network policy is not enforced"
ok "pod outside the allow-list cannot reach postgres ($res)"
res=$(probe sample-web 80)
[ "$res" != "rc=0" ] || fail "an unlabelled pod reached sample-web directly"
ok "pod outside the allow-list cannot reach sample-web ($res)"

# --- RBAC least privilege ---
[ "$(can get pods --as=alice --as-group=forgelab:viewers)" = "yes" ] || fail "viewers should read pods"
[ "$(can delete pods --as=alice --as-group=forgelab:viewers)" = "no" ] || fail "viewers must not delete pods"
[ "$(can patch deployments --as=alice --as-group=forgelab:viewers)" = "no" ] || fail "viewers must not patch deployments"
[ "$(can get secrets --as=alice --as-group=forgelab:viewers)" = "no" ] || fail "viewers must not read secrets"
ok "viewer role is read-only and cannot read secrets"

[ "$(can delete pods --as=bob --as-group=forgelab:operators)" = "yes" ] || fail "operators should delete pods"
[ "$(can patch deployments --as=bob --as-group=forgelab:operators)" = "yes" ] || fail "operators should patch deployments"
[ "$(can get secrets --as=bob --as-group=forgelab:operators)" = "no" ] || fail "operators must not read secrets"
[ "$(can create rolebindings --as=bob --as-group=forgelab:operators)" = "no" ] || fail "operators must not change RBAC"
[ "$(can create pods/exec --as=bob --as-group=forgelab:operators)" = "no" ] || fail "operators must not exec into pods"
ok "operator role can act on workloads but not on secrets, RBAC, or exec"

[ "$(can list pods --as=system:serviceaccount:$NS:sample-web)" = "no" ] || fail "the application service account must have no API access"
ok "application service account has no API permissions"

echo "kubernetes security smoke test passed"
