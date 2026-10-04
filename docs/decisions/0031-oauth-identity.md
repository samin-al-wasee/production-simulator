# ADR-0031: OAuth-only sign-in with database-backed sessions

**Status:** accepted
**Date:** 2026-10-04

## Context

Phase 13 gives ForgeLab users. The roadmap names OAuth sign-in with GitHub and Google and database-backed sessions; a broader draft also raised email-and-password. Credentials are the highest-risk data the product would hold, and the product has no passwords today. Identity must sit entirely in the application layer ([ADR-0030](0030-application-store-and-postgres.md)); the simulation core never learns a user exists.

## Decision

1. **Identity is OAuth 2.0 only**, the Authorization Code flow with PKCE, over two providers: GitHub and Google. Their client ids and secrets come from the environment (`GITHUB_CLIENT_ID`/`GITHUB_CLIENT_SECRET`, `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET`); a provider without credentials is not offered. **No passwords are stored, hashed, or accepted**, so there is no signup form, no reset flow, and no credential database to leak.
2. **A user is created or linked on first sign-in**, keyed by `(provider, provider_user_id)`. The provider's email and display name seed the profile; email is not a login key.
3. **Sessions are opaque random tokens** (256 bits from `crypto/rand`). Only the **SHA-256 hash** is stored, in the `sessions` table, with an expiry; the raw token travels only in the cookie. The cookie is `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, with a 30-day expiry. Logout deletes the session row, so a session is revocable server-side.
4. **The callback is verified** with a single-use `state` value and the PKCE verifier; neither is trusted from the client beyond the round trip.
5. **Authorization is per session:** handlers read the user id from the session, never from the request body or query. Every user-owned resource is filtered by that id, so one user cannot read or change another's data.
6. **The endpoints live under `/api/v1/auth/`** (`login`, `callback`, `logout`, and a current-user endpoint). The simulation core is untouched; the dashboard receives no credential.

## Consequences

### Positive

* No password database: the most common credential breach class does not exist here.
* Server-side sessions are revocable, unlike a bare JWT, and carry no signature secret to rotate.
* Adding a provider later is data, not a new credential path.

### Negative / Trade-offs

* Sign-in depends on the provider being reachable; there is no offline or password fallback.
* Users without a GitHub or Google account cannot sign in.
* Sessions require a store lookup per authenticated request (`ponytail:` sessions in Postgres; add a cache only if that lookup ever measures as a bottleneck).

## Alternatives Considered

* **Email and password** — rejected for this phase: it adds password hashing, verification, reset tokens, and a credential store, all of it attack surface, for capability OAuth already covers. It can return later as one more provider beside OAuth.
* **JWTs** — rejected: revocation needs a denylist, which is a session table in disguise, plus a signing secret.
* **Sessions in memory** — rejected: they vanish on redeploy and do not span replicas.

## References

* [ADR-0030](0030-application-store-and-postgres.md) (application store), [ADR-0014](0014-sandbox-only-platform.md) (Sandbox-only platform)
* [`ROADMAP.md`](../../ROADMAP.md) Phase 13, [`docs/security.md`](../security.md)
