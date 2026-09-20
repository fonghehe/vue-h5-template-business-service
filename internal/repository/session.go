package repository

import (
	"context"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"gorm.io/gorm"
)

func (r *UserRepository) CreateSession(ctx context.Context, session model.RefreshSession) error {
	session.ExpiresAt = session.ExpiresAt.UTC()
	if err := r.db.WithContext(ctx).Create(&session).Error; err != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not create session", err)
	}
	return nil
}

// RotateSession consumes one valid token and persists its replacement atomically.
// Concurrent consumers wait on the DELETE lock; exactly one can delete the row.
func (r *UserRepository) RotateSession(ctx context.Context, hash string, next model.RefreshSession) error {
	next.ExpiresAt = next.ExpiresAt.UTC()
	return WrapTransaction(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("token_hash = ? AND user_id = ? AND expires_at > ?", hash, next.UserID, time.Now().UTC()).Delete(&model.RefreshSession{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return apierr.Unauthorized("invalid or expired refresh session")
		}
		return tx.Create(&next).Error
	}))
}

func (r *UserRepository) RevokeSession(ctx context.Context, hash string) error {
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).Delete(&model.RefreshSession{}).Error; err != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not revoke session", err)
	}
	return nil
}

// CleanupSessions bounds each deletion so an expiry backlog cannot monopolize
// the worker or hold an unbounded write transaction.
func (r *UserRepository) CleanupSessions(ctx context.Context, limit int) error {
	subquery := r.db.Model(&model.RefreshSession{}).Select("token_hash").
		Where("expires_at <= ?", time.Now().UTC()).Order("expires_at").Limit(limit)
	if err := r.db.WithContext(ctx).Where("token_hash IN (?)", subquery).Delete(&model.RefreshSession{}).Error; err != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not clean expired sessions", err)
	}
	return nil
}
