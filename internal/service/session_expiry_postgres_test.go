package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"gorm.io/gorm/clause"
)

func TestPostgresRefreshRejectsSessionExpiredDuringLockWait(t *testing.T) {
	fixture := newPostgresCheckoutFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// The JWT remains valid; the persisted session expires during the row wait.
	// This avoids relying on JWT's one-second timestamp precision.
	issuer := fixture.services.Auth.issuer
	pair, err := issuer.Issue(fixture.userID, "checkout-user", "", nil, time.Minute, time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(2 * time.Second)
	session := model.RefreshSession{TokenHash: tokenHash(pair.RefreshToken), UserID: fixture.userID, ExpiresAt: expiresAt}
	if err := fixture.services.Auth.users.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	blocker := fixture.db.WithContext(ctx).Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	t.Cleanup(func() { blocker.Rollback() })
	var locked model.RefreshSession
	if err := blocker.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "token_hash = ?", session.TokenHash).Error; err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, _, err := fixture.services.Auth.Refresh(ctx, pair.RefreshToken); done <- err }()
	waitForCheckoutLocks(t, fixture.db, 1, nil)
	// Wait for a real time boundary, after first observing the database lock.
	timer := time.NewTimer(time.Until(expiresAt.Add(20 * time.Millisecond)))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	err = <-done
	var appError *apierr.Error
	if !errors.As(err, &appError) || appError.Code != apierr.CodeUnauthorized {
		t.Errorf("Refresh(session expired during lock wait) error = %v, want unauthorized", err)
	}
	var sessions []model.RefreshSession
	if err := fixture.db.Find(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].TokenHash != session.TokenHash {
		t.Errorf("Refresh(expired session) sessions = %+v, want original session only and no replacement", sessions)
	}
}
