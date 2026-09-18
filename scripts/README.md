# Scripts

Development and validation helpers. **Nothing is implemented yet.**

## Purpose

Small, focused helper scripts that make ForgeLab reproducible and pleasant to develop:

* Validation gates (manifest validation, docs-link checks)
* One-command local setup / teardown
* Generators (templates, scenarios, components)
* Reproducers used by scenarios

## Conventions

* One script per concern. Favor a few well-named helpers over a sprawling toolbox.
* Cross-platform where possible; prefer plain shell/PowerShell with no exotic dependencies.
* Never destructive by default; require an explicit `--yes` / `--force` for anything irreversible.
* No secrets — scripts must never read or write credentials.
* Adding a helper script warrants an entry under folder ownership (`docs/repository-structure.md`) and this README.