package httpapi

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
)

// pagination reads and clamps page and pageSize query parameters.
// An out-of-range value silently falls back to the default rather than
// erroring: a bad page number should show the first page, not a 400.
func pagination(c *gin.Context, defaultSize, maxSize int) (int, int) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), defaultSize)
	if pageSize > maxSize {
		pageSize = maxSize
	}
	return page, pageSize
}

// pathID parses an identifier supplied as a path parameter or query value.
func pathID(value string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("invalid id")
	}
	return uint(parsed), nil
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func optionalNonNegativeInt64(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return nil, errors.New("invalid non-negative integer")
	}
	return &parsed, nil
}

// serviceUnavailable reports a dependency outage. The underlying reason is
// logged by the caller but never exposed to the client.
func serviceUnavailable(cause error) error {
	return apierr.Unavailable("Service temporarily unavailable")
}
