#!/bin/sh
# Validate the Terraform cloud presets without credentials and without
# creating anything: formatting, provider install (no backend), and
# `terraform validate`. Requires terraform on PATH.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
command -v terraform >/dev/null 2>&1 || { echo "missing required tool: terraform" >&2; exit 1; }

for preset in aws gcp; do
  dir="$ROOT/environments/cloud/terraform/presets/$preset"
  echo "== $preset"
  terraform -chdir="$dir" fmt -check -diff
  terraform -chdir="$dir" init -backend=false -input=false >/dev/null
  terraform -chdir="$dir" validate
done
