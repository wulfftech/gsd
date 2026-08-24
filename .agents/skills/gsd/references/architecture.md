# GSD Architecture

## Purpose
Self-hosted household task & chore management (fork of Donetick, github.com/wulfftech/gsd). Core upstream: recurring chores with flexible scheduling, natural-language task creation, subtasks, assignee rotation, circles (household sharing), notifications (Telegram/Pushover/Discord/FCM).

## Fork additions
- **Projects**: task collections with assignees, due dates, points reward; any assignee completes, admins approve.
- **Rewards**: members redeem accumulated points for admin-defined rewards.
- **Dashboard**: next-24h due tasks + per-member team card (tasks, points, completions today).
- **Managed accounts**: circle admins create/rename/reset/delete lightweight logins without email.
- **Missed chore auto-advance** and admin approval badge.

## Stack
| Layer | Tech |
|---|---|
| Backend | Go 1.24, Gin, GORM, glebarez pure-Go SQLite driver (Postgres also supported) |
| Auth | gin-jwt, golang-jwt v4/v5, MFA via pquerna/otp |
| Frontend | React + Vite + MUI Joy UI + TanStack Query + Tailwind, i18n, Capacitor mobile builds |
| Docs | swaggo (swagger.json/yaml generated into docs/) |
| Container | Docker single-stage multi-build Dockerfile + docker-compose |

## Repo layout
- `main.go` — all wiring: config load, logging, DB, migration run, router setup, scheduled jobs.
- `config/` — viper-based config profiles selected by `DT_ENV` (default `selfhosted`); `selfhosted.yaml`.
- `internal/<feature>/` — one package per domain: auth, chore, circle, project, reward, user, device, label, subtask, filter, thing, points, notifier, realtime (WebSocket), mfa, events, storage. Feature modules follow the pattern `handler.go` / `api.go` / `model/` / `repo/`.
- `migrations/` — Go struct migrations (see migrations.md).
- `internal/database/` — GORM engine construction (sqlite/postgres switch), OTel tracing, GORM logging levels derived from app log level; `migration.go` glue.
- `frontend/` — Go package that embeds `dist/` and serves SPA when `server.serve_frontend` is true.
- `ui/` — actual frontend source. `npm run build-selfhosted` outputs to `frontend/dist` for embedding.
- `scripts/build.sh` — docker compose build wrapper stamping VERSION/COMMIT/BUILD_DATE from git.
- `CODE_REVIEW.md` — curated open findings re-verified against HEAD; consult before middleware/WS/security work.

## Env vars
`DT_ENV` (profile), `DT_SQLITE_PATH`, `TZ`. Health: `GET /api/v1/health`; build info: `GET /api/v1/resource` (`api_version`, `api_commit`).
