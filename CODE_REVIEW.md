# GSD — Open Review Findings

Originally a full-repo review dated 2026-08-07 (Go backend, React frontend, build/deploy/DB,
plus a live diagnosis of the Home Assistant integration). **Most of it has since been fixed.**

This document was re-verified against `HEAD` on **2026-08-23** and trimmed to what is still
true. Every item below was confirmed by reading the current source, not inferred from commit
messages. Resolved findings are listed in one line each at the bottom so the record survives
without reading as a live bug list.

Line numbers are accurate as of 2026-08-23 and will drift — locate code by symbol name.

---

## Still open

### 1. SQLite backup silently omits most of the database
**Severity: HIGH — data loss on restore**

`backupSQLiteDatabase` ([main.go:73](main.go)) runs before migrations on every startup and
copies **only** `donetick.db` via `os.Open`/`io.Copy`. It never runs
`PRAGMA wal_checkpoint` and never copies the `-wal`/`-shm` sidecars. Since WAL mode was
enabled ([internal/database/database.go:85](internal/database/database.go)), committed
transactions live in the `-wal` file until a checkpoint — so the backup omits them.

Confirmed on production 2026-08-23: after a clean `docker compose stop`, `donetick.db` was
600 K dated Aug 8 while `donetick.db-wal` was 2.6 M dated that day. **Every `.bak-*` file on
that host is a copy of a two-week-stale database.** The WAL does not checkpoint on shutdown
either, which suggests the app never closes the DB handle on SIGTERM.

**Fix:** `PRAGMA wal_checkpoint(TRUNCATE)` before copying, or use SQLite's online backup API;
and add a SIGTERM handler that closes the DB. Until then, back up all three files together
with the container stopped, and verify with `PRAGMA integrity_check` plus a row count against
the live DB.

### 2. `config.Info.Version` is never stamped — `/api/v1/resource` always reports `"dev"`
**Severity: MEDIUM**

There are two independent version variable sets. `main.Version`/`Commit`/`BuildDate`
([main.go:68](main.go)) *are* targeted by the Dockerfile's `-ldflags` and are logged at
startup. But `config.Version`/`Commit`/`BuildDate` ([config/config.go:18](config/config.go))
are a separate set, copied into `config.Info` by `LoadConfig`, and the Dockerfile's ldflags
never target `donetick.com/core/config.*`. That value is what
`GET /api/v1/resource` serves as `api_version`/`api_commit`
([internal/resource/handler.go:42](internal/resource/handler.go)).

Verified live 2026-08-23: `{"api_version":"dev","api_commit":"dev"}` on a build stamped
`80e9433`. Any client version-gate reading that endpoint sees `dev` forever.

**Fix:** add `-X donetick.com/core/config.Version=${VERSION}` (and Commit/BuildDate) to the
Dockerfile `go build` line, or have `LoadConfig` read the `main` values. Note
`/api/v1/resource` is unauthenticated and has no rate limiter despite one being passed to
`Routes` — worth deciding deliberately whether the version belongs there at all.

### 3. `TimeoutMiddleware` cannot interrupt a slow handler
**Severity: MEDIUM**

[internal/utils/middleware.go:59](internal/utils/middleware.go) sets a context deadline, but
the `ctx.Err() == context.DeadlineExceeded` check sits in a `defer` registered before
`c.Next()`, so it only evaluates after the handler has already returned. It relabels a
finished response as 504 rather than aborting anything in flight.

**This is a design decision, not a typo.** Making it real requires handlers to observe
`ctx.Done()`, which changes how handlers are written codebase-wide. Left deliberately.

### 4. WebSocket `CheckOrigin` defaults to `*`
**Severity: LOW**

The mechanism is now correct — [internal/realtime/handler.go:133](internal/realtime/handler.go)
reads `RealTimeConfig.AllowedOrigins` and denies unlisted origins. But the in-code default
([config/config.go:270](config/config.go)) and the shipped `config/selfhosted.yaml` both set
`["*"]`, and production is running with the wildcard (verified 2026-08-23). So an unmodified
deployment still accepts any Origin.

Low severity for the same reason as before: WS auth travels via query param/header, not a
cookie, so there is no forgeable cross-site credential to ride on. It becomes a real problem
the moment cookie auth is introduced. **This is a config posture choice, not a code bug** —
set a real origin list on any internet-facing instance.

### 5. Frontend: 45 react-query v5 calls still use the v4 array form
**Severity: MEDIUM**

The project is on `@tanstack/react-query` v5, where `invalidateQueries(['key'])` leaves
`filters.queryKey` undefined and therefore matches **every** query in the cache. The correct
form is `invalidateQueries({ queryKey: ['key'] })`. 45 bare-array calls remain across `ui/src`
(the chore/timer paths were converted; the rest were not).

Effect is over-invalidation rather than staleness — wasteful, and it masks genuine key-name
bugs. One such bug is still live: [ui/src/views/Chores/MyChores.jsx:1029](ui/src/views/Chores/MyChores.jsx)
invalidates `['circleMembers']`, a key registered nowhere (the real one is
`['allCircleMembers']`). It currently "works" only because the bare-array form invalidates
everything — two bugs cancelling out. Fixing the syntax without fixing the key name would
turn this into a visible staleness bug, so **fix both together.**

Same pattern in `CircleSettings.jsx`, `Settings.jsx`, and `ChildUserSettings.jsx` — correct
key names, v4 syntax.

### 6. Timer session list is correct by accident
**Severity: LOW**

[ui/src/views/Timer/TimerDetails.jsx:989](ui/src/views/Timer/TimerDetails.jsx) calls
`.sort()` directly on `timerData.pauseLog` — which mutates the react-query cached array in
place during render. `handleDeleteSession` then indexes into that same array, so the indices
*do* align and the correct session is deleted. But the correctness depends entirely on the
in-place mutation having already happened.

Two things to fix: sort a copy (`[...timerData.pauseLog].sort(...)`), and key
`showMoreInfoId` on a stable id rather than a positional index
([TimerDetails.jsx:1011](ui/src/views/Timer/TimerDetails.jsx)). Mutating cached query data
is also a react-query anti-pattern independent of this bug.

### 7. Migrations still run through three uncoordinated systems
**Severity: LOW-MEDIUM**

[main.go:352](main.go) runs GORM `AutoMigrate`, the custom registry, then `MigrationScripts`
in a fixed, undocumented order. `Down()` only fires as best-effort cleanup on failure — there
is no operator-invoked rollback path ([migrations/base.go:72](migrations/base.go)).

Materially improved since the original review: a backup now runs first (see item 1 for why it
isn't sufficient), the duplicate `migrate.Exec` is gone, and `migrations/migrations_test.go`
now provides smoke and idempotency coverage.

### 8. Smaller open items

- **Assets route has no auth middleware.** The S3 signed-URL check is now a real HMAC
  ([internal/storage/signer_s3.go:82](internal/storage/signer_s3.go)), but
  `GET /api/v1/assets/*filepath` still has no auth layer, and `S3Storage.Get()` is still
  unimplemented. Worth adding defence in depth before `Get()` is ever finished.
- **`PointsRepository.CreatePointsHistory` is dead code.** Chore completion/approval now
  write `points_history` rows directly via `tx.Create`, closing the data-integrity gap — but
  the shared helper at [internal/points/repo/repository.go:18](internal/points/repo/repository.go)
  is still called from nowhere. Route the four award paths through it or delete it.
- **`RedeemReward` cross-circle error text is misleading.** Status codes are now correct
  (403/400, no longer 500), but a wrong-circle `userId` yields "Insufficient points to redeem
  this reward". Cosmetic.
- **`/welcome` Landing page still carries Donetick branding** — `Footer.jsx` and
  `GettingStarted.jsx` link to docs.donetick.com, the donetick GitHub, Docker Hub, subreddit,
  support email, and the `com.donetick.app` Play Store listing. The route is still registered
  in `RouterContext.jsx`. (Note: `donetick.com` *hostname checks* in backend code are
  deliberate — they gate self-hosted vs official-instance behaviour — and must stay.)
- **eapi chore creation still can't express five frequency types** — interval,
  days_of_the_week, day_of_the_month, adaptive, and trigger need scheduling metadata the
  endpoint doesn't accept, and are explicitly rejected with 400. By design, not oversight.
- **Unused frontend dependencies** — `caniuse-lite`, `dotenv`, `esm`, `reusify` are declared
  in `ui/package.json` with zero import sites.
- **No route-level code splitting** — 46 static top-level imports in `RouterContext.jsx`, no
  `React.lazy`.
- **No frontend test tooling** — no vitest/jest, zero test files under `ui/`. CI lints and
  builds the frontend but does not test it.

---

## Resolved since 2026-08-07

Verified fixed against `HEAD` on 2026-08-23. Kept as a one-line record so this ground isn't
re-covered.

**Security** — cross-tenant IDOR on project task reject/mark-done (circle scoping added to
both repo queries) · eapi chore completion bypassing `RequireApproval` · `completedBy`
impersonation on complete *and* skip (admin/manager gate added to both) · S3 signed-URL stub
returning `true` (now a real HMAC check) · API tokens never expiring and shown in full (expiry
column, throttled `LastUsedAt`, masked in listings) · missing rate limiting on reward/thing
eapi groups · `JoinCircle` nil-pointer on a bogus invite code (now 404) · `refresh_token`
cookie missing `SameSite`.

**Correctness** — completion-window inversion and its nil-deref · non-atomic reward redemption
double-spend (conditional `UPDATE` + `RowsAffected`) · project reopen double-awarding points
(clawback + idempotency guard) · two inconsistent points ledgers (reconciled, with migration
`20260808b`) · concurrent last-task approval skipping project completion (row lock) ·
`skipChore` missing its nil guard · `week_of_month`/`week_of_quarter` ignoring chore timezone ·
ignored `ResetSubtasksCompletion` errors · `ConnectionPoolStats` copying a mutex (DTO split) ·
missing `points_history` rows on chore awards · duplicate `SubTask` delete.

**Frontend** — reward mutations not checking `resp.ok` (all five, plus `onError` at every call
site and error display in the modal) · circle-member invalidation using a nonexistent key
(fixed in `CircleSettings`/`Settings`; see open item 5 for the remaining site) · task approval
not refreshing points · managed-account changes not refreshing members · project task delete
lacking a status guard and edit dropping `status`/`completedBy` · overdue badge firing ~16 h
early in UTC+8 · `useUserProfile` not checking `resp.ok` and never handling 403 · timer
"Delete Session" stub · `JoinCircle` operator precedence · silent "Remove Member" failure ·
missing `aria-label`s · PWA manifest branding · hardcoded `app.donetick.com` redirect · dead
`Sharing.jsx`/`SharingSettings.jsx` · ungated `/test` route.

**Infra** — README not warning that the JWT placeholder crash-loops the container · dead
`config/selfhosted.env` (deleted) · no SQLite WAL / busy_timeout / pool limits · Dockerfile
non-reproducible install, unpinned base image, root user · no CI (GitHub Actions now runs
build/vet/lint/test on both backend and frontend) · test coverage 4 → 11 packages, covering
every area the original review prioritised · version stamping plumbed through
`scripts/build.sh` → compose → Dockerfile → startup log (but see open item 2).

**HA integration** — the original root cause (completion-window inversion + missing
`gin.Recovery()`) is fixed, as is the `/eapi/v1/things` trailing-slash mismatch. Since then:
`api_tokens.name` is unique per-user instead of globally, `GET /eapi/v1/chore/:id` exists, a
read-only Projects eapi surface exists, chore creation accepts frequency and assignee, and
`webhookURL` is settable via `PUT /api/v1/user/webhook` (API-only — no UI field yet).

---

## Current state

```
go vet ./...     clean
go test ./...    11 packages ok, 0 failing
                 config · external/payment · external/payment/repo · internal/auth
                 internal/chore · internal/database · internal/mfa · internal/storage
                 internal/user · internal/utils · migrations
```

*Re-verify against current `HEAD` before acting on anything here.*
