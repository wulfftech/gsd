package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newWALTestDB opens a scratch SQLite database in WAL mode and writes enough
// rows to leave them sitting in the -wal sidecar. It deliberately stays well
// under SQLite's 1000-page auto-checkpoint threshold so the data does NOT
// reach the main .db file on its own — that gap is the whole point of these
// tests.
func newWALTestDB(t *testing.T, dbPath string, rows int) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open scratch database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying SQL DB: %v", err)
	}
	// Windows will not let t.TempDir() remove a file that still has an open
	// handle, so release it once the test finishes.
	t.Cleanup(func() { sqlDB.Close() })

	var mode string
	if err := sqlDB.QueryRow("PRAGMA journal_mode=WAL;").Scan(&mode); err != nil {
		t.Fatalf("failed to enable WAL mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, want wal — the rest of this test is meaningless without it", mode)
	}

	if _, err := sqlDB.Exec("CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT);"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	for i := 0; i < rows; i++ {
		if _, err := sqlDB.Exec("INSERT INTO widgets (name) VALUES (?);", fmt.Sprintf("widget-%d", i)); err != nil {
			t.Fatalf("failed to insert row %d: %v", i, err)
		}
	}

	return db
}

func walSize(t *testing.T, dbPath string) int64 {
	t.Helper()

	info, err := os.Stat(dbPath + "-wal")
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("failed to stat WAL file: %v", err)
	}
	return info.Size()
}

func countWidgets(t *testing.T, dbPath string) int {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open %s: %v", dbPath, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying SQL DB: %v", err)
	}
	defer sqlDB.Close()

	var count int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM widgets;").Scan(&count); err != nil {
		t.Fatalf("failed to count rows in %s: %v", dbPath, err)
	}
	return count
}

func findBackup(t *testing.T, dir, dbPath string) string {
	t.Helper()

	matches, err := filepath.Glob(dbPath + ".bak-*")
	if err != nil {
		t.Fatalf("failed to glob for backup: %v", err)
	}
	if len(matches) != 1 {
		entries, _ := os.ReadDir(dir)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected exactly 1 backup file, found %d; directory contains %v", len(matches), names)
	}
	return matches[0]
}

// TestBackupSQLiteDatabase_CapturesWALContents is the regression test for the
// backup being unusable: it copied only the main .db file, which in WAL mode
// can be missing an unbounded amount of recent data. On the deployed instance
// this meant every .bak-* file was a copy of a two-week-stale database.
func TestBackupSQLiteDatabase_CapturesWALContents(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "donetick.db")
	const rows = 50

	db := newWALTestDB(t, dbPath, rows)

	// Precondition: the writes really are stranded in the WAL. If this fails,
	// SQLite checkpointed on its own and the test proves nothing.
	if walSize(t, dbPath) == 0 {
		t.Fatal("WAL is empty before backup — cannot demonstrate the bug this test covers")
	}

	t.Setenv("DT_SQLITE_PATH", dbPath)
	if err := backupSQLiteDatabase(nil, db); err != nil {
		t.Fatalf("backupSQLiteDatabase() error = %v, want nil", err)
	}

	backupPath := findBackup(t, dir, dbPath)
	if got := countWidgets(t, backupPath); got != rows {
		t.Errorf("backup contains %d rows, want %d — the WAL contents were not captured", got, rows)
	}
}

// TestCheckpointSQLiteWAL_TruncatesWAL covers the second half of the fix: the
// -wal file is reset to zero length, so it can't grow without bound across
// restarts and a plain file copy of the .db is complete.
func TestCheckpointSQLiteWAL_TruncatesWAL(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "donetick.db")

	db := newWALTestDB(t, dbPath, 50)

	before := walSize(t, dbPath)
	if before == 0 {
		t.Fatal("WAL is empty before checkpoint — nothing to truncate")
	}

	if err := checkpointSQLiteWAL(db); err != nil {
		t.Fatalf("checkpointSQLiteWAL() error = %v, want nil", err)
	}

	if after := walSize(t, dbPath); after != 0 {
		t.Errorf("WAL is %d bytes after TRUNCATE checkpoint (was %d), want 0", after, before)
	}
}

// TestCheckpointSQLiteWAL_MainFileIsSelfSufficient asserts the property that
// actually matters to an operator: after a checkpoint, copying donetick.db on
// its own — no sidecars — yields every committed row.
func TestCheckpointSQLiteWAL_MainFileIsSelfSufficient(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "donetick.db")
	const rows = 50

	db := newWALTestDB(t, dbPath, rows)

	if err := checkpointSQLiteWAL(db); err != nil {
		t.Fatalf("checkpointSQLiteWAL() error = %v, want nil", err)
	}

	// Copy the main file alone, exactly as a naive operator or the old backup
	// code would, and verify nothing is missing from it.
	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read database file: %v", err)
	}
	copyPath := filepath.Join(dir, "copy.db")
	if err := os.WriteFile(copyPath, data, 0o600); err != nil {
		t.Fatalf("failed to write copy: %v", err)
	}

	if got := countWidgets(t, copyPath); got != rows {
		t.Errorf("main-file-only copy has %d rows, want %d", got, rows)
	}
}

// TestBackupSQLiteDatabase_NoDatabaseIsNotAnError covers a fresh install, where
// there is nothing to back up yet and startup must not be blocked.
func TestBackupSQLiteDatabase_NoDatabaseIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "does-not-exist.db")

	t.Setenv("DT_SQLITE_PATH", dbPath)
	if err := backupSQLiteDatabase(nil, nil); err != nil {
		t.Fatalf("backupSQLiteDatabase() error = %v, want nil for a missing database", err)
	}

	if matches, _ := filepath.Glob(dbPath + ".bak-*"); len(matches) != 0 {
		t.Errorf("created %d backup files for a nonexistent database, want 0", len(matches))
	}
}
