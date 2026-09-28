# tls-downgrade-attempt

Category: security · Status: implemented

**Defensive drill.** Run it only against your own local lab (localhost). Never point these commands at systems you do not own.

## Goal

Confirm a client cannot be silently downgraded to plain HTTP or to obsolete TLS versions, and that an untrusted certificate is rejected.

## Components involved

Reverse Proxy, TLS / PKI (local CA). Setup: `make up-secure`.

## Expected symptoms

- `http://localhost:8080/...` answers `301` to an `https://` URL.
- A client without the local CA rejects the certificate.
- A TLS 1.1 handshake fails; TLS 1.2 and 1.3 succeed.
- Responses carry `Strict-Transport-Security`.

## Investigation

```sh
curl -sI http://localhost:8080/healthz | head -3
curl -s https://localhost:8443/healthz                       # fails: unknown CA
curl -s --cacert environments/local/security/certs/ca.crt --tlsv1.1 --tls-max 1.1 https://localhost:8443/healthz   # fails
curl -sI --cacert environments/local/security/certs/ca.crt https://localhost:8443/healthz | grep -i strict
```

## Success criteria

- Each downgrade path fails or redirects as described.
- You can explain what HSTS adds beyond the redirect.
