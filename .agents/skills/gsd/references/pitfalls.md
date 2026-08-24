# GSD Pitfalls & Open Issues

Source: README.md warnings + CODE_REVIEW.md (re-verified against HEAD 2026-08-23).

## Setup / config
- Shipped `jwt.secret` placeholder causes container crash at startup — must replace with real random value (`openssl rand -base64 32`) in `config/selfhosted.yaml`.
- WebSocket `RealTimeConfig.AllowedOrigins` defaults to `["*"]` in both code default and shipped yaml; set a real origin list for internet-facing instances.
- Module path is `donetick.com/core` even though directory is `gsd` — imports, go commands all use the donetick path.

## Build / deploy
- Plain `docker compose build` stamps VERSION/COMMIT/DATE as "dev" → indistinguishable deploys. Always use `./scripts/build.sh`.
- ui build scripts (`build-cf`, `build-selfhosted`) delete package-lock.json and force-install linux-x64 rollup/swc natives — platform-specific hackery; expect churn.
- Frontend source lives in `ui/`, not `frontend/`; `frontend/dist` is the embedded output.

## Database
- Never copy live `donetick.db` while app is running — WAL sidecar holds recent writes; stop container first or use pre-migration snapshots.
- Pre-migration `.bak-*` snapshots are never pruned; clean up manually. They are not backups.
- Migration ordering is pure string sort on ID — malformed IDs silently reorder execution.
- Legacy ledger bug history: points_redeemed vs points dual-ledger allowed overspending pre-2026-08-08; `points` now means spendable balance everywhere. Don't reintroduce reads of `points_redeemed`.

## Code-level known issues (deliberate, from CODE_REVIEW.md)
- `TimeoutMiddleware` (internal/utils/middleware.go) cannot actually interrupt slow handlers — deadline check sits in a defer before c.Next(); it relabels finished responses as 504. Design decision; handlers must observe ctx.Done() themselves if you change this.
- WS auth travels via query param/header, not cookies — safe-ish with wildcard Origin, but becomes exploitable if cookie auth is ever introduced.
