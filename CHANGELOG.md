# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0-rc.1] — 2026-05-20

The first release candidate for **拾光VPS (Shiguang VPS) 1.0** — a single Go
binary that bundles subscription aggregation, agent monitoring, a visual
operator pipeline, and a 10-channel notification system. Built to drop into
the gap between `sub-store`, Nezha, and Uptime Kuma without forcing users to
swap clients or migrate data.

### Added

- **Multi-protocol subscription aggregation.** Parse 12 protocol URIs
  (vmess / vless / ss / ssr / trojan / hysteria / hysteria2 / tuic /
  wireguard / anytls / socks5 / naive) from URL, uploaded YAML, or manual
  entry. Re-emit as Clash / Clash Meta YAML.
- **sub-store compatibility layer.** Drop-in `/download/:name?token=…`
  endpoint so existing mihomo / Clash Verge Rev clients pick up new
  config without any client change.
- **Visual operator pipeline (差异化亮点 #1).** Drag-and-drop editor
  combining `filter / sort / dedupe / regex-rename / map / output` operators
  with an inline YAML view, dry-run preview, and per-operator diff debug
  output. 100 nodes × 6 operators executes in well under 1 ms locally.
- **Node table with batch TCPing.** 200-node concurrent reachability probe
  (concurrency 50, < 5 s) with per-node latency, loss, last-seen, and
  one-click "ping again" — the practical observability layer above raw
  configuration data.
- **Agent + Nezha v2 compatibility (差异化亮点 #2).** A < 10 MB Go agent
  reports CPU / memory / disk / netio / load / connections over WebSocket.
  Existing Nezha v2 agents only swap the hub URL — no migration tooling
  required. Hub stays available behind Cloudflare via heartbeat-tolerant
  presence (30 s heartbeat, 90 s grace, two missed probes before offline).
- **10-channel notification system (差异化亮点 #2 cont.).** Telegram,
  Discord, Slack, Email (SMTP), Bark, Gotify, custom Webhook, ServerChan,
  PushDeer, IFTTT — including a bidirectional Telegram bot with five
  commands (`/nodes`, `/refresh`, `/traffic`, `/silent`, `/help`) and
  inline keyboards. Event-channel routing, per-event templates, dedupe,
  and live SSE stream for the in-app inbox.
- **Goja JavaScript hooks.** Operator-level sandboxed scripts (op-count
  ceiling 1 × 10⁷) with edit-and-run inside the UI. Auto-disable after
  three consecutive failures, with a user-facing notification.
- **Traffic accounting + monthly reset.** Daily aggregation of agent
  netio counters, monthly auto-reset on a configurable cutoff day, and
  threshold-based alerts (80 % / 100 % budgets) that hand off to the
  notify system.
- **Silent mode.** All non-whitelisted paths return a convincing
  `nginx/1.18.0` 404; the real UI lives under a 32-character hex prefix
  (`/_app/<prefix>/…`) that an admin can rotate. Helpful for shaking off
  drive-by scanners and keeping a single VPS multi-tenant-ish.
- **OTA self-update.** Panel-driven binary update from GitHub Releases
  with SHA-256 verification, `wal_checkpoint(TRUNCATE)` before swap, and
  `.bak` rollback. Graceful restart (drains outstanding requests) so SSH
  is no longer the only path to ship a fix.
- **Backups + restore.** One-click `.tar.gz` export of the SQLite WAL
  database plus settings (`PRAGMA wal_checkpoint(TRUNCATE)` first), and
  a restore flow that gates write traffic during the swap.
- **TOTP 2FA + recovery codes.** Per-user 2FA with `pquerna/otp`, ten
  single-use base32 recovery codes, brute-force protector with sliding
  window, and a session-revocation API.
- **Multi-locale UI (zh-CN, en, ja, ko).** Hand-written zh-CN, machine-
  translated and human-reviewed en / ja / ko. CI enforces key parity and
  bans hard-coded CJK in source files.
- **Dashboard + Cmd+K palette.** A 6-card overview with live counts,
  trend sparklines, and recent events, plus a Cmd+K palette that fuses
  page navigation, quick actions, admin operations, and cross-collection
  resource search.

### Engineering highlights

- Single Go binary, < 30 MB, no cgo. SQLite via `modernc.org/sqlite` ships
  in the binary; the web assets are embedded with `go:embed`.
- Five-OS / arch GitHub Actions matrix (linux-amd64, linux-arm64,
  darwin-amd64, darwin-arm64, windows-amd64) — every release fans out
  build + test + smoke.
- 50+ unit/integration tests on the Go side, 100+ Vitest cases in the
  React app, plus Playwright E2E specs covering the five primary user
  journeys (login + 2FA, subscription wizard, pipeline edit, notify
  config, agent connect).
- `scripts/check-size.sh` and `scripts/check-i18n.sh` keep file size and
  locale parity from drifting; the latter runs `--strict` in CI from
  1.0.0-rc.1 onwards.
- Performance baseline (see `docs/perf-baseline.md`): pipeline run < 1 ms
  for 100 nodes × 6 ops, TCPing 200 nodes / conc 50 < 5 s, RSS < 50 MB
  after 60 s idle.

### Known limitations

- No HTTPS long-polling fallback for the agent WebSocket; Cloudflare
  users need WebSocket support enabled. Tracked as a v1.x item.
- TCPing only — UDP / ICMP not supported yet.
- Telegram bot defaults to webhook delivery. Long-polling fallback for
  CN-only hubs is planned for v1.1.
- Subscription producers limited to Clash / Clash Meta; Surge / Quantumult X
  exports are on the roadmap.

---

## Releasing

To publish 1.0.0-rc.1 from a clean main:

```bash
git tag v1.0.0-rc.1
git push origin v1.0.0-rc.1
# triggers .github/workflows/release.yml which builds the 5-platform matrix
# and uploads the archives + checksums to the GitHub Release.
```

Release notes for the human-facing announcement live in
[`docs/release-notes-1.0.0-rc.1.md`](docs/release-notes-1.0.0-rc.1.md).

---

[1.0.0-rc.1]: https://github.com/shiguang-vps/shiguang-vps/releases/tag/v1.0.0-rc.1
