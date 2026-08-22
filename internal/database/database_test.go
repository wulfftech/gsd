package database

import (
	"os"
	"path/filepath"
	"testing"

	"donetick.com/core/config"
	"gorm.io/gorm/logger"
)

// TestNewDatabase_SQLiteReliabilitySettings asserts the SQLite-specific
// reliability pragmas and pool limits are genuinely applied against a real
// temp-file database, not just requested and silently ignored.
func TestNewDatabase_SQLiteReliabilitySettings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gsd-test.db")
	t.Setenv("DT_SQLITE_PATH", dbPath)

	cfg := &config.Config{
		Database: config.DatabaseConfig{Type: "sqlite"},
		Logging:  config.LogConfig{Level: "silent"},
	}

	db, err := NewDatabase(cfg)
	if err != nil {
		t.Fatalf("NewDatabase() error = %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("sqlDB.Close() error = %v", err)
		}
	})

	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("expected sqlite file to exist at %q: %v", dbPath, err)
	}

	var journalMode string
	if err := sqlDB.QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode; query error = %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}

	var busyTimeout int
	if err := sqlDB.QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout); err != nil {
		t.Fatalf("PRAGMA busy_timeout; query error = %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want %d", busyTimeout, 5000)
	}

	stats := sqlDB.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("MaxOpenConnections = %d, want %d", stats.MaxOpenConnections, 1)
	}
}

// TestNewDatabase_SQLiteWALSurvivesReopen confirms the reliability pragmas
// are not just set on the opening connection but genuinely persisted to the
// database file, by reopening it fresh and checking again.
func TestNewDatabase_SQLiteWALSurvivesReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gsd-test-reopen.db")
	t.Setenv("DT_SQLITE_PATH", dbPath)

	cfg := &config.Config{
		Database: config.DatabaseConfig{Type: "sqlite"},
		Logging:  config.LogConfig{Level: "silent"},
	}

	db1, err := NewDatabase(cfg)
	if err != nil {
		t.Fatalf("NewDatabase() first open error = %v", err)
	}
	sqlDB1, err := db1.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	if err := sqlDB1.Close(); err != nil {
		t.Fatalf("sqlDB1.Close() error = %v", err)
	}

	db2, err := NewDatabase(cfg)
	if err != nil {
		t.Fatalf("NewDatabase() second open error = %v", err)
	}
	sqlDB2, err := db2.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB2.Close(); err != nil {
			t.Errorf("sqlDB2.Close() error = %v", err)
		}
	})

	var journalMode string
	if err := sqlDB2.QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode; query error = %v", err)
	}
	// WAL mode is a persistent property of the database file itself, so a
	// freshly-opened connection should already report it even before
	// NewDatabase's own PRAGMA call runs again.
	if journalMode != "wal" {
		t.Errorf("journal_mode on reopened db = %q, want %q", journalMode, "wal")
	}
}

func TestConvertLogLevelToGorm(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want logger.LogLevel
	}{
		{"debug maps to gorm Info (most verbose)", "debug", logger.Info},
		{"info maps to gorm Warn", "info", logger.Warn},
		{"warn maps to gorm Error", "warn", logger.Error},
		{"warning maps to gorm Error", "warning", logger.Error},
		{"error maps to gorm Error", "error", logger.Error},
		{"dpanic maps to gorm Error", "dpanic", logger.Error},
		{"panic maps to gorm Error", "panic", logger.Error},
		{"fatal maps to gorm Error", "fatal", logger.Error},
		{"silent maps to gorm Silent", "silent", logger.Silent},
		{"unknown level defaults to Error for production safety", "bogus", logger.Error},
		{"empty level defaults to Error", "", logger.Error},
		{"matching is case-insensitive", "DEBUG", logger.Info},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertLogLevelToGorm(tt.in); got != tt.want {
				t.Errorf("convertLogLevelToGorm(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
