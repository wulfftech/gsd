package migrations

import (
	"context"
	"testing"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	sModel "donetick.com/core/external/payment/model"
	filterModel "donetick.com/core/internal/filter/model"
	lModel "donetick.com/core/internal/label/model"
	nModel "donetick.com/core/internal/notifier/model"
	pModel "donetick.com/core/internal/points"
	projModel "donetick.com/core/internal/project/model"
	rModel "donetick.com/core/internal/reward/model"
	storageModel "donetick.com/core/internal/storage/model"
	stModel "donetick.com/core/internal/subtask/model"
	tModel "donetick.com/core/internal/thing/model"
	uModel "donetick.com/core/internal/user/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Set up the basic schema using AutoMigrate, just like database.Migration() does.
	// This is needed because the data migrations in the migrations package expect these tables to exist.
	if err := db.AutoMigrate(
		uModel.User{},
		chModel.Chore{},
		chModel.ChoreHistory{},
		cModel.Circle{},
		cModel.UserCircle{},
		chModel.ChoreAssignees{},
		nModel.Notification{},
		uModel.UserPasswordReset{},
		sModel.StripeCustomer{},
		sModel.StripeSubscription{},
		sModel.StripeSession{},
		sModel.StripeInvoice{},
		sModel.RevenueCatEvent{},
		sModel.RevenueCatSubscription{},
		sModel.Subscription{},
		uModel.MFASession{},
		uModel.UserSession{},
		tModel.Thing{},
		tModel.ThingChore{},
		tModel.ThingHistory{},
		uModel.APIToken{},
		uModel.UserNotificationTarget{},
		lModel.Label{},
		chModel.ChoreLabels{},
		projModel.Project{},
		projModel.ProjectAssignee{},
		projModel.ProjectTask{},
		filterModel.Filter{},
		Migration{},
		pModel.PointsHistory{},
		rModel.Reward{},
		rModel.RewardRedemption{},
		stModel.SubTask{},
		storageModel.StorageFile{},
		storageModel.StorageUsage{},
		chModel.TimeSession{},
		uModel.UserDeviceToken{},
	); err != nil {
		t.Fatalf("failed to set up test database schema: %v", err)
	}

	return db
}

func TestMigrations_SmokeTest(t *testing.T) {
	db := setupTestDB(t)

	// Run all registered migrations
	err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("migrations.Run() failed: %v", err)
	}

	// Verify that the migrations table was created and populated
	var migrationCount int64
	db.Model(&Migration{}).Count(&migrationCount)
	if migrationCount == 0 {
		t.Errorf("expected migrations to be applied, but migration count is 0")
	}

	// Verify expected core tables exist. GORM uses "users", "chores", etc. (plural form from model names)
	expectedTables := []string{
		"users",
		"chores",
		"chore_histories",  // GORM pluralizes to chore_histories
		"circles",
		"user_circles",
		"labels",
		"projects",
	}

	for _, tableName := range expectedTables {
		if !db.Migrator().HasTable(tableName) {
			t.Errorf("expected table %q to exist after migrations, but it does not", tableName)
		}
	}
}

func TestMigrations_CanRunMultipleTimes(t *testing.T) {
	db := setupTestDB(t)

	// Run migrations first time
	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("first Run() failed: %v", err)
	}

	var countAfterFirst int64
	db.Model(&Migration{}).Count(&countAfterFirst)

	// Run migrations second time - should skip already-applied migrations
	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("second Run() failed: %v", err)
	}

	var countAfterSecond int64
	db.Model(&Migration{}).Count(&countAfterSecond)

	// Count should remain the same since migrations are already applied
	if countAfterFirst != countAfterSecond {
		t.Errorf("expected migration count to remain %d after second run, but got %d", countAfterFirst, countAfterSecond)
	}
}
