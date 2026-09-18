#!/usr/bin/env bash
set -euo pipefail

echo "ForgeLab devcontainer ready"
go version
node --version
npm --version
docker compose version
jq --version