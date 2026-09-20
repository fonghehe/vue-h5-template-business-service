package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/auth"
	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The service's failure paths can be tested without SQL or external services.
type failingAuthStore struct {
	AuthStore
	err error
}

func (s failingAuthStore) FindByUsername(context.Context, string) (model.User, error) {
	return model.User{}, s.err
}
func (s failingAuthStore) FindByID(context.Context, uint) (model.User, error) {
	return model.User{}, s.err
}
func TestAuthDoesNotHideStorageFailureAsInvalidCredentials(t *testing.T) {
	cfg := config.Load()
	issuer := auth.New(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience)
	cause := errors.New("private database failure")
	svc := NewAuthService(cfg, failingAuthStore{err: apierr.Wrap(apierr.CodeInternal, "database request failed", cause)}, issuer)
	_, _, err := svc.Login(context.Background(), "user", "password", true)
	require.ErrorIs(t, err, cause)
	pair, err := issuer.Issue(1, "user", "User", nil, time.Minute, time.Hour, true)
	require.NoError(t, err)
	_, _, err = svc.Refresh(context.Background(), pair.RefreshToken)
	require.ErrorIs(t, err, cause)
}

func TestRefreshRotationAndLogoutPreventReplay(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	_, first, err := fixture.services.Auth.Login(ctx, "user", "123456", true)
	require.NoError(t, err)
	_, second, err := fixture.services.Auth.Refresh(ctx, first.RefreshToken)
	require.NoError(t, err)
	_, _, err = fixture.services.Auth.Refresh(ctx, first.RefreshToken)
	require.Error(t, err)
	require.NoError(t, fixture.services.Auth.Logout(ctx, second.RefreshToken))
	require.NoError(t, fixture.services.Auth.Logout(ctx, second.RefreshToken))
	_, _, err = fixture.services.Auth.Refresh(ctx, second.RefreshToken)
	require.Error(t, err)
	var sessions []model.RefreshSession
	require.NoError(t, fixture.db.Find(&sessions).Error)
	assert.Empty(t, sessions)
}

func TestPostgresConcurrentRefreshHasExactlyOneWinner(t *testing.T) {
	db := newPostgresCommerceDB(t)
	require.NoError(t, db.Create(&model.User{Username: "user", PasswordHash: dummyHash, Roles: model.Roles{model.RoleUser}, Status: "active"}).Error)
	cfg := config.Load()
	svc := New(cfg, db, nil, nil, nil).Auth
	pair, err := svc.issuer.Issue(1, "user", "User", nil, time.Minute, time.Hour, true)
	require.NoError(t, err)
	session, err := svc.session(pair.RefreshToken, 1)
	require.NoError(t, err)
	require.NoError(t, svc.users.CreateSession(context.Background(), session))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var group sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 10)
	for range 10 {
		group.Add(1)
		go func() { defer group.Done(); <-start; _, _, err := svc.Refresh(ctx, pair.RefreshToken); results <- err }()
	}
	close(start)
	group.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else {
			var apiError *apierr.Error
			require.ErrorAs(t, err, &apiError)
			assert.Equal(t, apierr.CodeUnauthorized, apiError.Code)
		}
	}
	assert.Equal(t, 1, winners)
	var count int64
	require.NoError(t, db.Model(&model.RefreshSession{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestPostgresConcurrentMigrationsAreSerialized(t *testing.T) {
	db := newPostgresCommerceDB(t)
	require.NoError(t, db.Exec("DROP TABLE refresh_sessions").Error)
	require.NoError(t, db.Exec("DELETE FROM schema_migrations WHERE version IN ?", []string{"008_create_refresh_sessions", "009_enforce_commerce_constraints"}).Error)
	var group sync.WaitGroup
	results := make(chan error, 4)
	for range 4 {
		group.Add(1)
		go func() { defer group.Done(); results <- database.Migrate(db) }()
	}
	group.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.NoError(t, database.CheckSchema(db))
	var count int64
	require.NoError(t, db.Table("schema_migrations").Where("version = ?", "008_create_refresh_sessions").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCleanupOnlyDeletesBoundedExpiredSessions(t *testing.T) {
	fixture := newCommerceFixture(t)
	sessions := []model.RefreshSession{
		{TokenHash: "expired-1", UserID: fixture.userID, ExpiresAt: time.Now().UTC().Add(-time.Hour)},
		{TokenHash: "expired-2", UserID: fixture.userID, ExpiresAt: time.Now().UTC().Add(-time.Minute)},
		{TokenHash: "live", UserID: fixture.userID, ExpiresAt: time.Now().UTC().Add(time.Hour)},
	}
	require.NoError(t, fixture.db.Create(&sessions).Error)
	require.NoError(t, fixture.services.Auth.users.CleanupSessions(context.Background(), 1))
	var stored []model.RefreshSession
	require.NoError(t, fixture.db.Order("token_hash").Find(&stored).Error)
	require.Len(t, stored, 2)
	assert.Equal(t, "expired-2", stored[0].TokenHash)
	assert.Equal(t, "live", stored[1].TokenHash)
}

func TestLegacyPriceRequiresACompleteDecimalAmount(t *testing.T) {
	for _, price := range []string{".", "1.", ".1", "1.234", "NaN", "-1", "1..2", ""} {
		assert.False(t, isNumeric(price), price)
	}
	for _, price := range []string{"0", "1", "12.30", "1.2"} {
		assert.True(t, isNumeric(price), price)
	}
}
