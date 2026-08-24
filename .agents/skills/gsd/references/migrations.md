# GSD Migration Workflow

Migrations are Go structs in `/mnt/d/Code/gsd/migrations/`, run automatically at app startup from `main.go` (`migrations.Run(context.Background(), db)` after a SQLite snapshot).

## Structure of a migration

```go
package migrations

type MyChange20260901 struct{}

func (m MyChange20260901) ID() string          { return "20260901_my_change" }
func (m MyChange20260901) Description() string { return "What it does" }

func (m MyChange20260901) Up(ctx context.Context, db *gorm.DB) error {
    return db.Transaction(func(tx *gorm.DB) error { /* ... */ })
}

func (m MyChange20260901) Down(ctx context.Context, db *gorm.DB) error { /* rollback */ }

func init() { Register(MyChange20260901{}) }
```

## Rules
1. **ID format**: `YYYYMMDD[letter]_slug`. `migrations.Run` sorts lexicographically — same-day follow-ups get letter suffixes (see `20260808_add_api_token_expiry...` and `20260808b_retire_points_redeemed_ledger`). A malformed ID can reorder execution.
2. Registration happens via `init()` + `Register(...)`; no central list to edit.
3. Applied IDs are recorded in the `Migration` table (`db.AutoMigrate(&Migration{})` creates it). Re-runs skip applied IDs.
4. On failure, `Up`'s error triggers `Down` (best-effort rollback), then startup aborts.
5. Wrap data changes in `db.Transaction`.
6. SQLite safety net: before running, the app checkpoints WAL and copies `donetick.db` → `donetick.db.bak-<timestamp>`. Snapshots accumulate forever — prune manually. Restore by stopping the container and copying the bak over the db.
7. Test files: `migrations/migrations_test.go`, `internal/database/migration_test.go` — extend them when adding nontrivial migrations.

## Existing migrations (as of 2026-08)
Label→labels_v2, chat_id→notification_target, frequency/notification metadata v2, completed_at→performed_at, notification templates, chores-private default, nth_day_of_month→week_of_month, api-token expiry + composite unique, points_redeemed ledger retirement.
