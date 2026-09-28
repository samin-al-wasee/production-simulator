#!/bin/sh
# Create (or reuse) the local kind cluster and deploy the ForgeLab base
# manifests. Requires docker, kind, and kubectl on PATH.
# Usage: scripts/k8s-up.sh [overlay-dir]   (default: base)
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
K8S="$ROOT/environments/cloud/kubernetes"
TARGET="${1:-$K8S/base}"
CLUSTER=forgelab
INGRESS_NGINX_URL="https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.11.3/deploy/static/provider/kind/deploy.yaml"
METRICS_SERVER_URL="https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.7.2/components.yaml"

for tool in docker kind kubectl; do
  command -v "$tool" >/dev/null 2>&1 || { echo "missing required tool: $tool" >&2; exit 1; }
done

if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  kind create cluster --config "$K8S/kind-config.yaml"
fi
kubectl config use-context "kind-$CLUSTER" >/dev/null

docker build -t forgelab/sample-web:local "$ROOT/applications/sample-web"
kind load docker-image forgelab/sample-web:local --name "$CLUSTER"

if ! kubectl get ns ingress-nginx >/dev/null 2>&1; then
  kubectl apply -f "$INGRESS_NGINX_URL"
fi
kubectl -n ingress-nginx rollout status deploy/ingress-nginx-controller --timeout=180s

if ! kubectl -n kube-system get deploy metrics-server >/dev/null 2>&1; then
  kubectl apply -f "$METRICS_SERVER_URL"
  # kind kubelets use self-signed certificates.
  kubectl -n kube-system patch deploy metrics-server --type=json \
    -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
fi
kubectl -n kube-system rollout status deploy/metrics-server --timeout=180s

# The namespace and secret are created outside the manifests so credentials
# never live in git.
kubectl create namespace forgelab --dry-run=client -o yaml | kubectl apply -f -
if ! kubectl -n forgelab get secret forgelab-db >/dev/null 2>&1; then
  kubectl -n forgelab create secret generic forgelab-db \
    --from-literal=POSTGRES_USER="${POSTGRES_USER:-forgelab}" \
    --from-literal=POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-forgelab}" \
    --from-literal=POSTGRES_DB="${POSTGRES_DB:-forgelab}"
fi

kubectl apply -k "$TARGET"
kubectl -n forgelab rollout status statefulset/postgres --timeout=180s
echo "forgelab is up: curl http://localhost:8081/version"
