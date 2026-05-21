# 拾光VPS 1.0.0-rc.1 — Release Notes

> _"sub-store + Nezha + Uptime Kuma in one < 30 MB Go binary."_

This is the first **1.0 release candidate** of 拾光VPS (shiguang-vps). It
contains every feature targeted at the 1.0 milestone; we publish it as
`rc.1` so existing self-hosters can stress it before the GA tag lands.

---

## Highlights

### 1. Visual Operator Pipeline (差异化亮点 #1)

Drag-and-drop editor with six operators — **filter / sort / dedupe /
regex-rename / map / output** — plus an inline YAML view and a per-step
diff preview. 100 nodes × 6 operators executes in **~ 130 µs** on an
arm64 laptop, well below our 500 ms budget.

<!-- screenshot: pipeline-editor.png — canvas + parameter panel + YAML view -->
<!-- screenshot: pipeline-preview.png — debug mode with per-operator diff -->

### 2. 10-Channel Notification System (差异化亮点 #2)

Telegram, Discord, Slack, Email (SMTP), Bark, Gotify, custom Webhook,
ServerChan, PushDeer, IFTTT — plus a **bidirectional Telegram bot** with
five inline commands. The notification engine deduplicates, applies
per-event templates, and streams live events to the in-app inbox via
SSE.

<!-- screenshot: notify-channels.png — channel matrix with status pills -->
<!-- screenshot: notify-tg-bot.png — Telegram chat with /nodes inline keyboard -->

### 3. Sub-store + Nezha drop-in compatibility

- **Sub-store clients** (mihomo, Clash Verge Rev, …) work without any
  config change — point them at `/download/:name?token=…` and you're done.
- **Nezha v2 agents** join the hub with nothing but a URL swap; v2
  fields are preserved verbatim, unknown fields warn but don't drop.

### 4. Single-binary, single-database deployment

- **< 30 MB** Go binary embedding the React 19 frontend.
- **SQLite-only** persistence (`modernc.org/sqlite`, no cgo). No
  external Postgres, Redis, or asset CDN.
- **< 50 MB RSS** after 60 s idle.

<!-- screenshot: dashboard-overview.png — six stat cards + sparklines -->

### 5. Built for self-hosters

- **Silent mode** keeps the UI behind a 32-hex prefix; non-whitelisted
  paths reply with a convincing nginx 404.
- **OTA self-update** from the panel with SHA-256 + WAL checkpoint + bak
  rollback.
- **Cmd+K** palette unifies navigation, mutations, admin ops, and
  cross-collection search.
- **4 locales** (zh-CN, en, ja, ko) with CI-enforced key parity.

<!-- screenshot: cmd-k-palette.png — the unified command palette -->
<!-- screenshot: settings-silent-mode.png — rotate-prefix button + warning -->

---

## What's inside

| Module | What it does |
|--------|--------------|
| **M-USER** | bcrypt password + TOTP 2FA + recovery codes + session revoke |
| **M-SUB** | 12 protocol parsers + URL / YAML / manual sources + ACL4SSR + Clash producer |
| **M-NODE** | node list + batch TCPing (200 / conc 50 / < 5 s) + last-seen / loss |
| **M-PIPE** | 6 operators + YAML codec + dry-run preview + per-op diff debug |
| **M-RULE** | DNS + rules + rule-providers in three editor modes (form / YAML / wizard) |
| **M-SCRIPT** | goja sandbox with op-count ceiling + auto-disable on repeat failure |
| **M-AGENT** | Go agent (< 10 MB) + Nezha v2 compat hub + WS presence + commands |
| **M-TRAFFIC** | daily aggregation + monthly reset + threshold alerts |
| **M-NOTIFY** | 10 channels + per-event templates + dedupe + SSE inbox + Telegram bot |
| **M-OPS** | silent mode + settings + backup + OTA self-update |

---

## Upgrade notes

This is the **first public release** — there is no upgrade path required.
Fresh installs only. If you've been running pre-release builds against the
same data directory, please:

1. Stop the server.
2. Take a backup (`Settings → Backup → Download`) before you upgrade.
3. Replace the binary and restart. The first start migrates the SQLite
   schema (idempotent).

---

## Known issues

- **WebSocket through CDN.** Cloudflare free-tier connections may drop
  every ~100 s. The presence layer tolerates two missed heartbeats, so
  short blips do not flip nodes to offline; ensure "WebSocket support"
  is enabled in your CDN config. HTTP long-polling fallback is planned
  for v1.1.
- **Telegram bot in mainland China hubs.** `api.telegram.org` is not
  reachable. Either set `HTTPS_PROXY=` to an outbound proxy, or use one
  of Bark / Server酱 / PushDeer (all unblocked in CN).
- **Pipeline `regex_rename` recompiles per operator.** Acceptable for
  the 100-node / 500 ms budget but on the radar — pool of compiled
  `*regexp.Regexp` lands in v1.1.
- **Subscription producers** ship Clash / Clash Meta only. Surge and
  Quantumult X exporters are tracked for v1.1.
- **OTA** does not yet roll back automatically on a startup crash; the
  `.bak` swap is manual until v1.1's self-healing first-boot probe.

See `CHANGELOG.md` for the full module list and engineering highlights.

---

## How to migrate

| You're coming from | Read this |
|--------------------|-----------|
| sub-store | [docs/user/migration-from-substore.md](user/migration-from-substore.md) |
| Nezha (any version) | [docs/user/migration-from-nezha.md](user/migration-from-nezha.md) |
| Brand new install | [docs/user/quickstart.md](user/quickstart.md) |

---

## Verifying the release

Every release artefact ships with a SHA-256 checksum and (where possible)
an attestation generated by `actions/attest-build-provenance`. To check
a downloaded archive:

```bash
shasum -a 256 -c shiguang-vps-1.0.0-rc.1-linux-amd64.tar.gz.sha256
```

---

## Thank-yous

- **modernc.org/sqlite** for a cgo-free SQLite that ships on every Go
  platform.
- **pquerna/otp** for the TOTP implementation.
- **dop251/goja** for the JS runtime that powers the script hooks.
- **cmdk**, **react-flow**, **tanstack/router**, **tanstack/query** —
  the React stack the frontend leans on.
- The sub-store and Nezha communities for the protocols and field
  schemas this release stays compatible with.

If you find a bug, please file an issue with the request ID surfaced in
the error toast — every response carries one and it's a one-line lookup
in the server log.

---

## Release flow (maintainer reference)

```bash
# 1. Verify the working tree is clean and on main.
git status

# 2. Tag the release. The release.yml workflow watches v* tags.
git tag v1.0.0-rc.1
git push origin v1.0.0-rc.1

# 3. The 5-platform build matrix runs; on success the GitHub Release is
#    created automatically with the archives, checksums, and attestations
#    attached. The release notes body is sourced from this file.
```
