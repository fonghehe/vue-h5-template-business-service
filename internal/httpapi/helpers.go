package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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
	if page > math.MaxInt/pageSize {
		page = 1
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

// bindJSON accepts exactly one JSON value and hides decoder/validator internals.
// Empty bodies are allowed only on endpoints explicitly supporting cookie-only
// requests or an empty checkout payload.
func bindJSON(c *gin.Context, input any, optional bool, message string) error {
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(input); err != nil {
		if optional && errors.Is(err, io.EOF) {
			return nil
		}
		return apierr.Validation(message)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return apierr.Validation(message)
	}
	if err := binding.Validator.ValidateStruct(input); err != nil {
		return apierr.Validation(message)
	}
	return nil
}
