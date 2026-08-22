package database

import (
	"context"
	"testing"
	"time"

	pModel "donetick.com/core/external/payment/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&pModel.StripeCustomer{},
		&pModel.StripeSession{},
		&pModel.StripeSubscription{},
		&pModel.StripeInvoice{},
		&pModel.RevenueCatSubscription{},
		&pModel.RevenueCatEvent{},
		&pModel.Subscription{},
	); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}
	return db
}

func TestStripeDB_CustomerRoundTrip(t *testing.T) {
	db := openTestDB(t)
	stripeDB := NewStripeDB(db)
	ctx := context.Background()

	now := time.Now().UTC()
	saved, err := stripeDB.SaveCustomer(ctx, &pModel.StripeCustomer{
		CustomerID: "cus_123",
		UserID:     42,
		CircleID:   7,
		CreatedAt:  &now,
	})
	if err != nil {
		t.Fatalf("SaveCustomer() error = %v", err)
	}
	if saved.CustomerID != "cus_123" {
		t.Fatalf("SaveCustomer() returned CustomerID = %q, want %q", saved.CustomerID, "cus_123")
	}

	byUserID, err := stripeDB.GetCustomer(ctx, 42)
	if err != nil {
		t.Fatalf("GetCustomer() error = %v", err)
	}
	if byUserID.CustomerID != "cus_123" {
		t.Errorf("GetCustomer() CustomerID = %q, want %q", byUserID.CustomerID, "cus_123")
	}

	byCustomerID, err := stripeDB.GetCustomerByCustomerID(ctx, "cus_123")
	if err != nil {
		t.Fatalf("GetCustomerByCustomerID() error = %v", err)
	}
	if byCustomerID.UserID != 42 {
		t.Errorf("GetCustomerByCustomerID() UserID = %d, want %d", byCustomerID.UserID, 42)
	}
}

// TestStripeDB_GetCustomerByCustomerID_NotFound documents a deliberate
// asymmetry in this package: GetCustomer propagates gorm.ErrRecordNotFound to
// the caller, but GetCustomerByCustomerID swallows it and returns (nil, nil)
// instead (see repository.go). Callers of GetCustomerByCustomerID must check
// for a nil result, not just a non-nil error, or a "customer not found" will
// silently look like success.
func TestStripeDB_GetCustomerByCustomerID_NotFound(t *testing.T) {
	db := openTestDB(t)
	stripeDB := NewStripeDB(db)

	got, err := stripeDB.GetCustomerByCustomerID(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("GetCustomerByCustomerID() error = %v, want nil error for a not-found record", err)
	}
	if got != nil {
		t.Errorf("GetCustomerByCustomerID() = %+v, want nil", got)
	}
}

func TestStripeDB_UpdateSubscription_RequiresSubscriptionID(t *testing.T) {
	db := openTestDB(t)
	stripeDB := NewStripeDB(db)

	err := stripeDB.UpdateSubscription(context.Background(), &pModel.StripeSubscription{Status: "active"})
	if err == nil {
		t.Error("UpdateSubscription() with an empty SubscriptionID expected an error, got nil")
	}
}

func TestStripeDB_SessionLifecycle(t *testing.T) {
	db := openTestDB(t)
	stripeDB := NewStripeDB(db)
	ctx := context.Background()

	now := time.Now().UTC()
	if _, err := stripeDB.SaveSession(ctx, &pModel.StripeSession{
		SessionID:  "sess_1",
		CustomerID: "cus_1",
		UserID:     1,
		Status:     "open",
		CreatedAt:  &now,
	}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	if err := stripeDB.UpdateSession(ctx, "sess_1", "complete"); err != nil {
		t.Fatalf("UpdateSession() error = %v", err)
	}

	got, err := stripeDB.GetSession(ctx, "sess_1")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got.Status != "complete" {
		t.Errorf("GetSession() Status = %q, want %q", got.Status, "complete")
	}

	if err := stripeDB.DeleteSession(ctx, "sess_1"); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if _, err := stripeDB.GetSession(ctx, "sess_1"); err == nil {
		t.Error("GetSession() after DeleteSession() expected an error, got nil")
	}
}

func TestRevenueCatDB_EventExists(t *testing.T) {
	db := openTestDB(t)
	rcDB := NewRevenueCatDB(db)
	ctx := context.Background()

	exists, err := rcDB.EventExists(ctx, "evt_1")
	if err != nil {
		t.Fatalf("EventExists() error = %v", err)
	}
	if exists {
		t.Fatal("EventExists() = true before the event was ever saved")
	}

	if _, err := rcDB.SaveEvent(ctx, &pModel.RevenueCatEvent{
		EventID:        "evt_1",
		EventType:      "INITIAL_PURCHASE",
		AppUserID:      "5",
		EventTimestamp: time.Now().UTC(),
		ProcessedAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveEvent() error = %v", err)
	}

	exists, err = rcDB.EventExists(ctx, "evt_1")
	if err != nil {
		t.Fatalf("EventExists() error = %v", err)
	}
	if !exists {
		t.Error("EventExists() = false after the event was saved; webhook idempotency check would reprocess it")
	}
}

func TestRevenueCatDB_UpdateSubscription_RequiresOriginalTransactionID(t *testing.T) {
	db := openTestDB(t)
	rcDB := NewRevenueCatDB(db)

	err := rcDB.UpdateSubscription(context.Background(), &pModel.RevenueCatSubscription{Status: "active"})
	if err == nil {
		t.Error("UpdateSubscription() with an empty OriginalTransactionID expected an error, got nil")
	}
}

func TestRevenueCatDB_GetSubscriptionByAppUserID_OnlyActive(t *testing.T) {
	db := openTestDB(t)
	rcDB := NewRevenueCatDB(db)
	ctx := context.Background()

	now := time.Now().UTC()
	if _, err := rcDB.SaveSubscription(ctx, &pModel.RevenueCatSubscription{
		AppUserID:             "user-1",
		OriginalTransactionID: "orig_1",
		Status:                "expired",
		CreatedAt:             &now,
	}); err != nil {
		t.Fatalf("SaveSubscription() error = %v", err)
	}

	got, err := rcDB.GetSubscriptionByAppUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetSubscriptionByAppUserID() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetSubscriptionByAppUserID() = %+v, want nil for a user with only an expired subscription", got)
	}
}

func TestSubscriptionDB_UpdateSubscription_RequiresID(t *testing.T) {
	db := openTestDB(t)
	subDB := NewSubscriptionDB(db)

	err := subDB.UpdateSubscription(context.Background(), &pModel.Subscription{Status: "active"})
	if err == nil {
		t.Error("UpdateSubscription() with an empty ID expected an error, got nil")
	}
}

func TestSubscriptionDB_GetSubscriptionByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	subDB := NewSubscriptionDB(db)

	got, err := subDB.GetSubscriptionByID(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("GetSubscriptionByID() error = %v, want nil error for a not-found record", err)
	}
	if got != nil {
		t.Errorf("GetSubscriptionByID() = %+v, want nil", got)
	}
}

// TestSubscriptionDB_GetSubscriptionByUserID_ExcludesCancelledAndPicksNewest
// guards the query that gates paid-feature access: it must ignore cancelled
// subscriptions and, when more than one active row exists for a user, return
// the most recently created one rather than an arbitrary row.
func TestSubscriptionDB_GetSubscriptionByUserID_ExcludesCancelledAndPicksNewest(t *testing.T) {
	db := openTestDB(t)
	subDB := NewSubscriptionDB(db)
	ctx := context.Background()

	older := time.Now().UTC().Add(-48 * time.Hour)
	newer := time.Now().UTC().Add(-1 * time.Hour)

	// ExternalSubscriptionID must be distinct per row: (provider,
	// external_subscription_id) is a unique index, and it defaults to "" if
	// left unset, which would collide across these three rows.
	if _, err := subDB.SaveSubscription(ctx, &pModel.Subscription{
		ID:                     "sub_cancelled",
		UserID:                 99,
		Status:                 "cancelled",
		ExternalSubscriptionID: "ext_cancelled",
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveSubscription(cancelled) error = %v", err)
	}
	if _, err := subDB.SaveSubscription(ctx, &pModel.Subscription{
		ID:                     "sub_active_old",
		UserID:                 99,
		Status:                 "active",
		ExternalSubscriptionID: "ext_active_old",
		CreatedAt:              older,
		UpdatedAt:              older,
	}); err != nil {
		t.Fatalf("SaveSubscription(active_old) error = %v", err)
	}
	if _, err := subDB.SaveSubscription(ctx, &pModel.Subscription{
		ID:                     "sub_active_new",
		UserID:                 99,
		Status:                 "active",
		ExternalSubscriptionID: "ext_active_new",
		CreatedAt:              newer,
		UpdatedAt:              newer,
	}); err != nil {
		t.Fatalf("SaveSubscription(active_new) error = %v", err)
	}

	got, err := subDB.GetSubscriptionByUserID(ctx, 99)
	if err != nil {
		t.Fatalf("GetSubscriptionByUserID() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetSubscriptionByUserID() = nil, want the newer active subscription")
	}
	if got.ID != "sub_active_new" {
		t.Errorf("GetSubscriptionByUserID() ID = %q, want %q (most recently created active subscription)", got.ID, "sub_active_new")
	}
}

func TestSubscriptionDB_GetSubscriptionByUserID_NoActiveSubscriptionReturnsNilNil(t *testing.T) {
	db := openTestDB(t)
	subDB := NewSubscriptionDB(db)
	ctx := context.Background()

	if _, err := subDB.SaveSubscription(ctx, &pModel.Subscription{
		ID:        "sub_cancelled_only",
		UserID:    5,
		Status:    "cancelled",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveSubscription() error = %v", err)
	}

	got, err := subDB.GetSubscriptionByUserID(ctx, 5)
	if err != nil {
		t.Fatalf("GetSubscriptionByUserID() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetSubscriptionByUserID() = %+v, want nil when the user has no active subscription", got)
	}
}

func TestStripeDB_GetSubscriptionByAccountID_Join(t *testing.T) {
	db := openTestDB(t)
	stripeDB := NewStripeDB(db)
	ctx := context.Background()

	now := time.Now().UTC()
	if _, err := stripeDB.SaveCustomer(ctx, &pModel.StripeCustomer{
		CustomerID: "cus_join",
		UserID:     123,
		CreatedAt:  &now,
	}); err != nil {
		t.Fatalf("SaveCustomer() error = %v", err)
	}
	if _, err := stripeDB.SaveSubscription(ctx, &pModel.StripeSubscription{
		SubscriptionID: "sub_join",
		CustomerID:     "cus_join",
		Status:         "active",
		CreatedAt:      &now,
	}); err != nil {
		t.Fatalf("SaveSubscription() error = %v", err)
	}

	got, err := stripeDB.GetSubscriptionByAccountID(ctx, 123)
	if err != nil {
		t.Fatalf("GetSubscriptionByAccountID() error = %v", err)
	}
	if got.SubscriptionID != "sub_join" {
		t.Errorf("GetSubscriptionByAccountID() SubscriptionID = %q, want %q", got.SubscriptionID, "sub_join")
	}
}
