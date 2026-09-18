# Service Template

Starting point for a new service/component inside a ForgeLab domain.

## Contents (planned)

* `README.md` — purpose, provided services, dependencies, status
* `config.example.yaml` — example configuration
* Component-specific IaC (Docker-only in `local`; Helm/manifests for `cloud`)

## Usage

1. Copy this folder to `components/<domain>/<service-name>/`.
2. Fill in the service README per `AGENTS.md` §9.
3. Add a row to `docs/component-catalog.md`.

Status: planned — available when the first component phase lands.