package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// isNotFound reports whether err is GORM's "no rows" sentinel.
func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// isDuplicate reports whether err is a unique constraint violation. Detection
// is driver specific because the drivers surface different error shapes.
func isDuplicate(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "constraint failed")
}
