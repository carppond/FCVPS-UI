# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please report it privately:

1. **DO NOT** open a public GitHub issue
2. Open a private security advisory via GitHub: **Security → Advisories → New draft security advisory**
3. Or email the maintainer (see repo profile)

Please include:
- A description of the vulnerability
- Steps to reproduce
- Affected version / commit hash
- Suggested fix if you have one

We'll respond within 7 days.

## Threat Model

This is a **self-hosted single-tenant** admin panel. The threat model assumes:

- The hub is deployed behind HTTPS (Nginx + Let's Encrypt)
- The operator trusts the server it runs on
- Multi-user mode (admin/user roles) provides logical separation but not strong isolation — both roles share the same SQLite database

## Known Limitations

### SSH Credentials Stored in Plaintext

VPS asset SSH passwords and private keys are stored as plain TEXT in SQLite (`vps_assets.ssh_password`, `vps_assets.ssh_private_key`).

**Risk**: Anyone with read access to `data/shiguang.db` (server compromise, backup leak) can recover all managed VPS SSH credentials.

**Mitigation**:
- Treat `data/` as a sensitive directory (mode 700, owned by service user)
- Encrypt backups
- Consider not storing production SSH passwords — use SSH key authentication and store the key on the mobile device only (Keychain/Keystore) in a future release
- A future migration may move to encrypted-at-rest with a key derived from the bootstrap secret

### Mobile App Transport Security (ATS)

The iOS app sets `NSAllowsArbitraryLoads: true` and Android `usesCleartextTraffic: true`. This is required to support self-hosted instances on any URL/port the user provides.

**Risk**: HTTP downgrade attack on hostile Wi-Fi if the user enters `http://` by mistake.

**Mitigation**: Always configure your hub URL with `https://` and a valid TLS certificate.

## Security Features

- **Authentication**: bcrypt (Go default cost) password hashing
- **Sessions**: 256-bit random tokens, stored as SHA-256 hashes
- **2FA**: TOTP support with recovery codes
- **Brute force protection**: Rate-limited login (5 attempts per hour per IP+username)
- **SSRF protection**: All outbound HTTP (subscription sync, rule-set fetch, webhook delivery) blocks RFC1918, loopback, link-local, and multicast addresses
- **Script sandbox**: goja JS runtime with `require`, `eval`, `fetch`, `setTimeout`, `process`, `fs`, etc. all blocked. 5-second CPU time limit
- **SQL**: All queries parameterized; no string concatenation
- **CSRF**: Bearer-token authentication makes cross-site requests authentication-impossible by default
- **Admin gating**: All `/api/admin/*` endpoints require `role=admin`
- **Silent mode**: Optional URL prefix that returns nginx-style 404 for any request without the secret entry path
- **Audit log**: All admin actions logged

## What's NOT in scope

- Multi-tenant isolation (this is single-tenant by design)
- DoS protection (deploy behind a CDN/WAF)
- Side-channel attacks
