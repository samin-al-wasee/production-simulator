# Templates

Reusable starting points for anything a user plugs into ForgeLab. Templates make the platform fast to adopt: give a manifest, a service, or an application a head start instead of a blank page. **Nothing is implemented yet.**

| Folder | Purpose |
|---|---|
| `application/` | Starting point for a new user application to plug into ForgeLab |
| `service/` | Starting point for a new service/component inside a domain |
| `manifests/` | Starting point for an application manifest |

## Conventions

* Templates are plain files with placeholders, not code generators.
* Placeholder tokens follow `<TOKEN>` style and the template README lists them.
* A template must be valid against the relevant schema/conventions when filled in.
* Adding a template updates the folder table above.