package logging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/requestctx"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DatabaseLogger records errors and slow queries without SQL text or bound
// parameters, which may contain credentials or customer data.
type DatabaseLogger struct {
	logger *slog.Logger
	level  gormlogger.LogLevel
}

func NewDatabaseLogger(logger *slog.Logger) gormlogger.Interface {
	return DatabaseLogger{logger: logger, level: gormlogger.Warn}
}
func (l DatabaseLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	l.level = level
	return l
}
func (l DatabaseLogger) Info(ctx context.Context, format string, args ...any) {
	if l.level >= gormlogger.Info {
		l.logger.InfoContext(ctx, fmt.Sprintf(format, args...), "requestId", requestctx.ID(ctx))
	}
}
func (l DatabaseLogger) Warn(ctx context.Context, format string, args ...any) {
	if l.level >= gormlogger.Warn {
		l.logger.WarnContext(ctx, fmt.Sprintf(format, args...), "requestId", requestctx.ID(ctx))
	}
}
func (l DatabaseLogger) Error(ctx context.Context, format string, args ...any) {
	if l.level >= gormlogger.Error {
		l.logger.ErrorContext(ctx, fmt.Sprintf(format, args...), "requestId", requestctx.ID(ctx))
	}
}
func (l DatabaseLogger) Trace(ctx context.Context, begin time.Time, _ func() (string, int64), err error) {
	elapsed := time.Since(begin)
	if l.level >= gormlogger.Error && err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		l.logger.ErrorContext(ctx, "database query failed", "requestId", requestctx.ID(ctx), "durationMs", elapsed.Milliseconds(), "error", err)
	} else if l.level >= gormlogger.Warn && elapsed > 200*time.Millisecond {
		l.logger.WarnContext(ctx, "slow database query", "requestId", requestctx.ID(ctx), "durationMs", elapsed.Milliseconds())
	}
}
