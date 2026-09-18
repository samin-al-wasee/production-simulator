# Applications

Plugged-in sample and user applications. **Nothing is implemented yet.**

## What belongs here

One directory per application: `applications/<name>/`, self-contained and clearly documented. Applications are **inputs to the platform** — they demonstrate how a real app gets declared in a manifest and run in ForgeLab. They are never part of the platform core.

Representative shapes ForgeLab should accept:

* Monolith
* Server-rendered application
* SPA + API
* Microservices
* Event-driven architecture
* Background workers
* Distributed systems

## Conventions

* An application owns its own `README.md` (what it is, stack, ports, how to run it standalone).
* It is declared through an application manifest (see `manifests/README.md`).
* Sample applications are optional; ForgeLab never requires a shipped app.

## Adding one

1. Create `applications/<name>/`.
2. Write the app's `README.md`.
3. Add a manifest for it under `manifests/`.
4. Reference it from the environment(s) that use it.
5. Update this file's catalog if you're adding a permanent sample.

## Planned samples (catalog)

| Name | Shape | Status |
|---|---|---|
| — | — | (none yet — Phase 1+) |