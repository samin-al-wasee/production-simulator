# Application Template

Starting point for a new application to plug into ForgeLab.

## Contents (planned)

* `README.md` — what the app is, stack, ports, how it runs standalone
* `Dockerfile` — container image definition (Phase 1)
* `application.example.yaml` — the app's manifest referencing this folder

## Usage

1. Copy this folder to `applications/<name>/`.
2. Replace `<APP_NAME>`, `<IMAGE>`, and `<PORT>` placeholders.
3. Declare the app in a manifest under `manifests/`.

Status: planned — available when the first runnable environment phase lands.