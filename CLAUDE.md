# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

拾光VPS (`shiguang-vps`) — self-hosted Clash subscription aggregation + VPS asset management + agent monitoring + notifications, shipped as one Go binary + React web + Expo mobile app.

## Repository layout

Four cooperating components in one repo:

- `cmd/server/` + `internal/` — the **hub** (Go HTTP server + SQLite).
- `cmd/agent/` — the **agent** (lightweight Go probe; reports CPU/MEM/Disk/NetIO over WebSocket; Nezha-v2 agent compatible).
- `web/` — React 19 + Vite 7 + TanStack Router/Query admin UI.
- `mobile/` — Expo SDK 56 / React Native app (iOS + Android), `expo-router` file routes.
- `migrations/` SQLite migrations · `scripts/` dev+deploy · `docs/` (architecture in `docs/03-architecture.md`, API in `docs/04-api-contract.md`).

## Common commands

```bash
# --- Local dev ---
go run ./cmd/server/                 # hub on :8080
cd web && pnpm install && pnpm dev   # web on :5173, proxies /api → :8080
./scripts/dev.sh                     # hub + web together
./scripts/dev-mobile.sh              # hub + Expo (JS hot-reload); --build = native build incl. widget

# --- Backend test / lint ---
go test -race -cover ./...                       # all
go test ./internal/handler/ -run TestRouter      # single test / package
gofumpt -l -w .                                  # format (stricter than gofmt)
golangci-lint run                                # lint (config: .golangci.yml)

# --- Web test / lint / build ---
cd web
pnpm test                            # vitest; `pnpm test -- src/lib/foo.test.ts` for one file
pnpm e2e                             # playwright
pnpm lint                            # eslint
pnpm build                           # tsc -b && vite build  (use this to type-check web)

# --- Mobile ---
cd mobile
npx tsc --noEmit                     # type-check (no test runner configured)
npx expo run:ios                     # build + run base app on simulator
EXPO_WIDGET=1 npx expo run:ios       # include the iOS home-screen widget (needs paid Apple acct)

# --- Cross-cutting guards (also run in CI) ---
./scripts/gen-types.sh               # diff Go types vs web api.ts (see "Type contract")
./scripts/check-size.sh              # file/function line-count limits
./scripts/check-i18n.sh              # 4 locales have equal key sets + no hardcoded CJK
```

## Backend architecture

Deliberately **stdlib-first** (see `docs/03-architecture.md` for the full rationale and the dependency red-lines):

- **No web framework** — `net/http` + a custom mux and middleware chain. The router is built in `internal/handler/router.go` (`NewRouter` takes a `Deps` struct whose fields are all nil-tolerant so individual handlers can be tested without a full DB). Middleware lives in `internal/handler/middleware/` (chain, cors, log, ratelimit, recover, audit, silent_mode).
- **No ORM** — `database/sql` + hand-written parameterized SQL. All SQL text is concentrated in `internal/storage/*_repo.go` (one repo per aggregate). `internal/storage/migrate.go` runs `migrations/*.sql` (embedded via `migrations/embed.go`) at startup.
- **Pure-Go SQLite** (`modernc.org/sqlite`, no cgo) — this is a hard constraint; it keeps `CGO_ENABLED=0` cross-compilation working. Never introduce a cgo SQLite driver.
- `internal/` modules by domain: `auth` (sessions, bcrypt, TOTP), `agent`/`nezha` (probe ingest + Nezha compat), `pipeline` (filter/sort/dedupe/regex-rename/map/output operator chain), `substore` (sub-store `/download/:name?token=` compatibility), `traffic`, `asset` (VPS records), `notify` (10 channels + Telegram bot), `firewall`, `ota`, `scriptengine` (goja JS sandbox), `shortlink`, `ratelimit`, `audit`, `config`, `logger`.
- **Startup** (`cmd/server/main.go`): config → slog → open SQLite + migrate → wire auth/token/totp → `EnsureAdmin` (first boot prints the initial admin password to the log — grep `ADMIN`) → router + background watchers → serve with 30s graceful shutdown.
- **Silent mode**: when enabled, every path returns an nginx-style 404 except routes under a random 32-hex prefix; the prefix + enabled flag live in `system_settings` and a background watcher hot-reloads them (`internal/handler/middleware/silent_mode.go`).
- The hub `embed`s migrations, the cross-compiled agent binaries (`internal/handler/install_script_handler.go`), and notify templates. **Web static is NOT embedded in the binary** — the Docker image runs nginx that serves `web/dist` and reverse-proxies the API to the hub (`docker/nginx.conf`); local dev uses the Vite proxy instead.

## Type contract (cross-cutting — read before touching API shapes)

The API request/response types exist in three places that must stay in sync:

- `internal/types/api.go` — Go source of truth.
- `web/src/types/api.ts` — **hand-maintained** TypeScript contract. Its header says "Code generated … DO NOT EDIT" but it is edited by hand per `docs/04-api-contract.md`; `scripts/gen-types.sh` runs `tygo` and only **diffs** the generated output against it (CI fails on drift) — it does not overwrite.
- `mobile/src/types/api.ts` — the mobile copy of the same contract.

When you change an API shape, update `internal/types/api.go`, `docs/04-api-contract.md`, `web/src/types/api.ts`, and `mobile/src/types/api.ts` together.

## Web conventions (enforced by scripts/CI — non-obvious)

Design is minimalist/Swiss, dark-first (`docs/_dev-cheatsheet.md` is the generated token reference):

- **Tokens only.** No bare `hex`/`rgb`/`hsl` literals and no arbitrary pixel values — use the Tailwind v4 token scale (`@theme` block in `web/src/styles/globals.css`). shadcn-derived primitives have a small whitelist of off-scale steps.
- **i18n always.** No hardcoded user-facing strings — use `t('namespace.key')`. All 4 locales must have identical key sets, and business `.ts/.tsx` must contain no hardcoded CJK (`check-i18n.sh`; native-language names in language switchers are whitelisted).
- **Four states.** Every data-driven component must handle normal / loading (Skeleton) / empty (EmptyState) / error (ErrorState).
- No gradients, no glassmorphism, 1px hairline borders, single accent color. File/function size caps via `check-size.sh`.

## Mobile conventions

- **Expo changed — read the versioned docs.** Before writing Expo/RN code, consult `https://docs.expo.dev/versions/v56.0.0/` (see `mobile/AGENTS.md`). When unsure about an `@expo/ui`/`expo-*` API, read the installed package's `.d.ts` under `node_modules/` — it is authoritative for the pinned version.
- **Optional-native pattern.** Native-only features (SSH terminal via `react-native-ssh-sftp`, the home-screen widget via `expo-widgets`) are absent in Expo Go and base builds. Code lazy-`require`s the native module inside `try/catch` and no-ops when unavailable — never import them at module top in a way that crashes Expo Go.
- **iOS widget** (`mobile/src/widgets/`, `mobile/src/lib/widget-sync.ts`): gated behind the `EXPO_WIDGET` env var in `app.config.js` because it needs an App Group (→ a paid Apple account); the default build omits it so free accounts can still build the base app. `APPLE_TEAM_ID` is injected via env, not committed. Inside a widget layout: the `'widget'` directive must be the **first statement in the layout function body**, and the function must be **self-contained** (only `@expo/ui/swift-ui` globals + React + JS builtins are available in the widget runtime — module-level vars/helpers are not serialized in).

## Conventions

- Commit messages: Conventional Commits with a scope, e.g. `feat(mobile):`, `fix(backend):`, `chore(dev):`.
- Go targets the version in `go.mod` (currently 1.26; golangci is pinned to the 1.24 toolchain). Web uses `pnpm` (frozen lockfile in CI); do not switch package managers.
