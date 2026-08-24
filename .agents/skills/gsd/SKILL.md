---
name: gsd
description: Work on the GSD Go/React chore tracker repo.
---

# GSD Skill

GSD ("Gettin' Shit Done") is Shannon's self-hosted fork of Donetick: a household
task/chore manager with recurring tasks, circles, projects with points, rewards,
dashboards, and managed accounts. Module path is `donetick.com/core`.

## When to Use

Any code change, build, test, migration, or deploy work inside `/mnt/d/Code/gsd`.

## Architecture

- **Backend** — Go 1.24, Gin, GORM over SQLite (`glebarez/sqlite`, pure-Go driver),
  wired with Uber `fx`. Entry point `main.go`.
- **internal/** — feature modules: `auth`, `chore`, `circle`, `database`, `device`,
  `email`, `events`, `filter`, `label`, `mfa`, `notifier` (telegram/pushover/
  discord/fcm), `points`, `project`, `realtime`, `resource`, `reward`, `storage`,
  `subtask`, `thing`, `user`. Each module typically has `handler.go`, `service/`,
  `repo/`, `model/`.
- **external/payment** — Stripe subscription integration.
- **Frontend** — lives in **`ui/`** (NOT the Go `frontend/` package, which only
  serves `ui/dist`). React + Vite + MUI Joy UI + TanStack Query; Capacitor for
  mobile shells; Cloudflare build target via wrangler.
- **config/** — Viper YAML profiles selected by `DT_ENV` (default `selfhosted`).
- **Port 2021**, served by Gin (API under `/api/v1` plus Swagger docs).

## Commands

```bash
# Backend
go run .            # dev server on :2021
go test ./...       # tests (main_test.go, per-package)

# Frontend
cd ui && npm install
npm run dev         # vite dev server
npm run lint        # eslint, zero-warning gate
npm run build-selfhosted   # production selfhosted bundle into ui/dist

# Deploy / release builds
./scripts/build.sh  # docker compose build with git version/commit/date stamps
docker compose up -d

# Verify deployed build
docker logs gsd | head -1           # version banner
GET /api/v1/resource                # api_version + api_commit
```

## Migrations

Three layers, applied on startup by `internal/database/migration.go`:

1. **GORM AutoMigrate** of all models (schema deltas).
2. **Embedded SQL migrations** (`internal/database/migrations/*.sql`) via
   `rubenv/sql-migrate`.
3. **Custom Go data migrations** in top-level `migrations/`: files named
   `YYYYMMDD[_b]_name.go` implementing `MigrationScript`
   (`ID/Description/Up/Down`), registered via `migrations.Register(...)` and
   sorted by ID. Add a new file here for data backfills; see existing scripts
   and `migrations/migrations_test.go`.

Before running migrations the app snapshots SQLite to
`donetick.db.bak-<timestamp>` (WAL checkpointed first) beside the DB.

## Key Files

- `main.go` — fx wiring, routes, middleware.
- `config/selfhosted.yaml` — config profile incl. required `jwt.secret`.
- `internal/database/migration.go` — migration entrypoint.
- `migrations/base.go` — MigrationScript interface + registry.
- `scripts/build.sh`, `scripts/check-time-utc.sh`.
- `docker-compose.yaml`, `Dockerfile`.
- `docs/swagger.*` — generated API docs.

## Pitfalls

- **jwt.secret placeholder**: shipped default secret makes the container crash
  on startup; must be replaced before first start.
- **Two "frontend" dirs**: edit React code in `ui/src`; `frontend/` is just the
  Go static-file handler. Rebuild `ui/dist` (`npm run build-selfhosted`) or your
  UI changes won't ship in Docker.
- **SQLite WAL**: never copy `donetick.db` while the app runs — recent writes
  live in `-wal`. Stop the container first, or use the auto-snapshots.
- **Backups aren't pruned** and are only as fresh as last restart — not a backup
  strategy.
- Use `./scripts/build.sh`, not bare `docker compose build`, so version/commit
  get stamped (bare builds read `dev`).
- Frontend platform-specific npm scripts (`build-cf`, `build-win`, `setup-m1`)
  force-install rollup/swc native binaries — pick the one matching the target OS.
- Timezone matters: chores are timezone-aware; `TZ` env and
  `scripts/check-time-utc.sh` exist for this reason.
- Repo has CRLF line endings (Windows checkout under WSL).
