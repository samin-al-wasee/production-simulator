# ADR-0033: Sign-in as the front door, and the My ForgeLab shell

**Status:** accepted
**Date:** 2026-10-04

## Context

[ADR-0032](0032-user-owned-resources.md) made saved games and learning progress
user-owned but deliberately deferred the sign-in UI: gating the API behind a
session "would leave the product unusable in the meantime," and the browser
tests ran the API without a store so they kept the file path they assert. Phase
13.1d is where that deferral is paid off — the product gets a sign-in front door
and a place to return to ("My ForgeLab").

Two constraints shape the design. The session cookie is `HttpOnly`,
`SameSite=Lax`, and host-only (`identity.setSessionCookie`). A `Lax` cookie is
**not** sent on a cross-site `fetch`, and `localhost:3001` and `127.0.0.1:8090`
are different sites — so a browser that talks to the API at a different origin
can neither receive the cookie on the callback nor send it afterwards. And the
OAuth `redirect_uri` is fixed by the backend as
`FORGELAB_PUBLIC_URL + "/api/v1/auth/callback/" + provider`
(`identity.redirectURI`), so `FORGELAB_PUBLIC_URL` decides which host the
callback — and therefore the session cookie — lands on.

## Decision

1. **Sign-in is optional, and it is the front door.** The header carries a
   sign-in control when providers are configured; an anonymous visitor can still
   create and drive an in-memory game. A `401` is a prompt to sign in, never a
   dead end. This keeps local, DB-free, and browser-test runs usable.
2. **The browser only ever talks to the dashboard origin.** A small Next.js
   route handler (`frontend/app/api/v1/auth/[...path]/route.ts`) proxies the
   backend's `/api/v1/auth/*` surface, copying the request cookie and every
   `Set-Cookie` and `Location` back verbatim. Game data keeps using the existing
   `/api/forgelab/*` rewrite. This is a same-origin flow, so the `SameSite=Lax`,
   host-only session cookie is first-party and reaches the dashboard.
3. **`FORGELAB_PUBLIC_URL` is the dashboard's public origin**, not the API's:
   the OAuth callback returns through the dashboard proxy. Each provider's
   registered redirect URI becomes
   `<FORGELAB_PUBLIC_URL>/api/v1/auth/callback/<provider>`.
4. **My ForgeLab** (`/me`) lists, resumes, and deletes a player's saved games
   and links to per-user learning progress. Server rendering forwards the
   incoming session so per-user data renders on the server (and the Learning
   path page stops showing empty progress for a signed-in player).
5. **`.forgelab` is retired for the product, not the core.** With a store, saves
   and progress live in Postgres; the no-store path (`-database=`; the CLI,
   DB-free development, and the anonymous browser suite) keeps the
   `.forgelab` files exactly as before (ADR-0030's "the core runs without a
   database").

## Consequences

### Positive

* One first-party cookie on the dashboard origin: no cross-site cookie, no
  `SameSite=None`, no CSRF token needed for the current surface.
* The core stays headless and DB-free; the CLI and the simulation tests never
  need a database, and the anonymous browser suite is untouched.
* A returned player has a home: their saved games and progress follow the
  account, not the browser.

### Negative / Trade-offs

* The dashboard now proxies the auth surface, and `FORGELAB_PUBLIC_URL`'s meaning
  changes (dashboard origin, not API origin) — a deployment change that must be
  made in `render.yaml`, `.devcontainer/docker-compose.yml`, and `.env` files.
* Sign-in UI cannot be exercised end to end by a real OAuth round trip in the
  browser tests; they seed a session against a database-backed API instead
  (`backend/cmd/seed-session`, gated by `FORGELAB_TEST_SESSION=1` and never in
  the deployed image).
* The `.forgelab` path still exists for no-store runs, so two persistence paths
  remain — but only one is the product.

## Alternatives Considered

* **Direct cross-origin calls with CORS and `credentials: "include"`** —
  rejected: a `SameSite=Lax` cookie is not sent on a cross-site fetch, and
  switching to `SameSite=None; Secure` would expose the session to cross-site
  requests and require CSRF protection.
* **A `rewrites()` entry for `/api/v1/auth/*` instead of a route handler** —
  rejected: external rewrites do not promise `Set-Cookie` fidelity across a
  302, and the whole flow depends on it; an explicit proxy is deterministic.
* **Require sign-in everywhere** — rejected: it breaks the CLI, DB-free
  development, and the browser tests, as ADR-0032 already found.
* **Point `FORGELAB_PUBLIC_URL` at the API and set the cookie on the API
  origin** — rejected: the dashboard could not read a cookie on another host.

## References

* [ADR-0030](0030-application-store-and-postgres.md) (application store),
  [ADR-0031](0031-oauth-identity.md) (identity),
  [ADR-0032](0032-user-owned-resources.md) (user-owned resources)
* [ADR-0010](0010-pipeline-simulation-and-dashboard.md) (same-origin rewrite)
* [`docs/architecture.md`](../architecture.md), [`docs/security.md`](../security.md),
  [`ROADMAP.md`](../../ROADMAP.md) Phase 13
