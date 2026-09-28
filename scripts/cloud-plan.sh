#!/bin/sh
# Produce a Terraform plan for a cloud preset and run it through the cost
# guard. This script NEVER applies anything: it stops after the check and the
# apply decision stays with a human.
#
# Usage: scripts/cloud-plan.sh <aws|gcp>
# Needs terraform, provider credentials in the environment, and a
# terraform.tfvars in the preset directory (see terraform.tfvars.example).
# `terraform plan` reads from the provider APIs but creates no resources.
set -eu

PRESET="${1:-}"
case "$PRESET" in
  aws|gcp) ;;
  *) echo "usage: $0 <aws|gcp>" >&2; exit 2 ;;
esac

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$ROOT/environments/cloud/terraform/presets/$PRESET"
command -v terraform >/dev/null 2>&1 || { echo "missing required tool: terraform" >&2; exit 1; }
[ -f "$DIR/terraform.tfvars" ] || { echo "missing $DIR/terraform.tfvars (copy terraform.tfvars.example)" >&2; exit 1; }

terraform -chdir="$DIR" init -input=false >/dev/null
terraform -chdir="$DIR" plan -input=false -out=tfplan
terraform -chdir="$DIR" show -json tfplan > "$DIR/tfplan.json"

cd "$ROOT/core"
go run ./cmd/forgelab costguard check -rules "$ROOT/environments/cloud/cost-guard.yaml" "$DIR/tfplan.json"
echo "plan saved at $DIR/tfplan (git-ignored). Applying is a manual decision: review it, then run terraform apply yourself."
