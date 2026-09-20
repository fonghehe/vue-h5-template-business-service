package model

import "time"

// RefreshSession stores only the SHA-256 digest of a signed refresh token.
// A consumed or logged-out token has no row and cannot create another session.
type RefreshSession struct {
	TokenHash string    `gorm:"primaryKey;size:64"`
	UserID    uint      `gorm:"index;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	CreatedAt time.Time
}
