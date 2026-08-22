package database

import (
	"testing"

	"donetick.com/core/config"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	return db
}

// TestMigration_AutoMigratesCoreSchema is a smoke test: if Migration() ever
// fails to create the schema the whole application fails to boot, so this
// just needs to catch a broken/removed model registration.
func TestMigration_AutoMigratesCoreSchema(t *testing.T) {
	db := openTestDB(t)

	if err := Migration(db); err != nil {
		t.Fatalf("Migration() error = %v", err)
	}

	expectedTables := []string{
		"users",
		"chores",
		"circles",
		"mfa_sessions",
		"user_sessions",
		"subscriptions",
		"storage_files",
		"storage_usages",
	}
	for _, tableName := range expectedTables {
		if !db.Migrator().HasTable(tableName) {
			t.Errorf("expected table %q to exist after Migration(), but it does not", tableName)
		}
	}
}

func TestMigration_CanRunMultipleTimesWithoutError(t *testing.T) {
	db := openTestDB(t)

	if err := Migration(db); err != nil {
		t.Fatalf("first Migration() call error = %v", err)
	}
	if err := Migration(db); err != nil {
		t.Fatalf("second Migration() call error = %v", err)
	}
}

// TestMigrationScripts_AppliesEmbeddedSQLMigrations exercises the embedded
// *.sql migrations (internal/database/migrations/*.sql) against a schema
// that AutoMigrate has already created, mirroring real startup order.
func TestMigrationScripts_AppliesEmbeddedSQLMigrations(t *testing.T) {
	db := openTestDB(t)
	if err := Migration(db); err != nil {
		t.Fatalf("Migration() error = %v", err)
	}

	cfg := &config.Config{Database: config.DatabaseConfig{Type: "sqlite"}}

	if err := MigrationScripts(db, cfg); err != nil {
		t.Fatalf("MigrationScripts() error = %v", err)
	}

	// Running again must be a no-op (sql-migrate tracks applied migrations),
	// not a duplicate-insert failure on the second run.
	if err := MigrationScripts(db, cfg); err != nil {
		t.Fatalf("second MigrationScripts() call error = %v", err)
	}
}

func TestMigrationScripts_UnsupportedDatabaseType(t *testing.T) {
	db := openTestDB(t)
	cfg := &config.Config{Database: config.DatabaseConfig{Type: "mysql"}}

	if err := MigrationScripts(db, cfg); err == nil {
		t.Error("MigrationScripts() with an unsupported database type expected an error, got nil")
	}
}
