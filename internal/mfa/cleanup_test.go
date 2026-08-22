package mfa

import (
	"context"
	"testing"
	"time"

	"donetick.com/core/config"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestUserRepo(t *testing.T) *uRepo.UserRepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&uModel.User{}, &uModel.MFASession{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	return uRepo.NewUserRepository(db, &config.Config{})
}

// TestCleanupExpiredMFASessions exercises the query the cleanup service relies
// on: it must remove sessions whose expiry is in the past while leaving
// still-valid sessions untouched.
func TestCleanupExpiredMFASessions(t *testing.T) {
	repo := newTestUserRepo(t)
	ctx := context.Background()

	expired := &uModel.MFASession{
		SessionToken: "expired-token",
		UserID:       1,
		AuthMethod:   "local",
		CreatedAt:    time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt:    time.Now().UTC().Add(-1 * time.Hour),
	}
	valid := &uModel.MFASession{
		SessionToken: "valid-token",
		UserID:       1,
		AuthMethod:   "local",
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(1 * time.Hour),
	}

	if err := repo.CreateMFASession(ctx, expired); err != nil {
		t.Fatalf("CreateMFASession(expired) error = %v", err)
	}
	if err := repo.CreateMFASession(ctx, valid); err != nil {
		t.Fatalf("CreateMFASession(valid) error = %v", err)
	}

	if err := repo.CleanupExpiredMFASessions(ctx); err != nil {
		t.Fatalf("CleanupExpiredMFASessions() error = %v", err)
	}

	if _, err := repo.GetMFASession(ctx, "expired-token"); err == nil {
		t.Error("expired MFA session was not removed by cleanup")
	}

	got, err := repo.GetMFASession(ctx, "valid-token")
	if err != nil {
		t.Fatalf("GetMFASession(valid) error = %v", err)
	}
	if got.SessionToken != "valid-token" {
		t.Errorf("GetMFASession(valid) returned token %q, want %q", got.SessionToken, "valid-token")
	}
}

func TestCleanupExpiredMFASessions_NoExpiredSessionsIsANoop(t *testing.T) {
	repo := newTestUserRepo(t)
	ctx := context.Background()

	valid := &uModel.MFASession{
		SessionToken: "still-valid",
		UserID:       1,
		AuthMethod:   "local",
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(1 * time.Hour),
	}
	if err := repo.CreateMFASession(ctx, valid); err != nil {
		t.Fatalf("CreateMFASession() error = %v", err)
	}

	if err := repo.CleanupExpiredMFASessions(ctx); err != nil {
		t.Fatalf("CleanupExpiredMFASessions() error = %v", err)
	}

	if _, err := repo.GetMFASession(ctx, "still-valid"); err != nil {
		t.Errorf("GetMFASession(still-valid) error = %v, want session to survive cleanup", err)
	}
}

// TestCleanupService_StartStop verifies the goroutine lifecycle: Start()
// launches the ticker loop and Stop() must be able to signal it to exit
// without blocking. The cleanup interval itself (60 minutes, hard-coded in
// NewCleanupService) is not exercised here — see the report for why.
func TestCleanupService_StartStop(t *testing.T) {
	repo := newTestUserRepo(t)
	svc := NewCleanupService(repo)

	svc.Start(context.Background())

	stopped := make(chan struct{})
	go func() {
		svc.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("CleanupService.Stop() did not return within 5s; possible deadlock between Start's goroutine and Stop's done channel send")
	}
}
