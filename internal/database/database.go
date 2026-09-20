// Package database opens the connection pool, applies versioned schema
// migrations and optionally loads demo data.
package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
)

// Options controls how the database is opened.
type Options struct {
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	RequireSchema   bool
	// AutoMigrate applies pending schema migrations.
	AutoMigrate bool
	// Seed loads demo data when tables are empty.
	Seed bool
	// Quiet suppresses GORM's own SQL logging; the access log already covers it.
	Quiet  bool
	Logger gormlogger.Interface
}

// OptionsFromConfig derives database options from service configuration.
func OptionsFromConfig(cfg config.Config) Options {
	return Options{
		Driver:          cfg.DatabaseDriver,
		DSN:             cfg.DatabaseURL,
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
		ConnMaxIdleTime: cfg.DBConnMaxIdleTime,
		RequireSchema:   !cfg.AutoMigrate,
		AutoMigrate:     cfg.AutoMigrate,
		Seed:            cfg.Seed,
	}
}

// Open connects to the database and prepares the schema.
func Open(opts Options) (*gorm.DB, error) {
	if err := ensureParentDir(opts.Driver, opts.DSN); err != nil {
		return nil, err
	}

	dialector, err := dialectorFor(opts.Driver, opts.DSN)
	if err != nil {
		return nil, err
	}

	queryLogger := opts.Logger
	if queryLogger == nil {
		queryLogger = gormlogger.Default.LogMode(logLevel(opts.Quiet))
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               queryLogger,
		TranslateError:       true,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access connection pool: %w", err)
	}
	initialized := false
	defer func() {
		if !initialized {
			_ = sqlDB.Close()
		}
	}()
	sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(opts.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(opts.ConnMaxIdleTime)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	db = db.WithContext(ctx)
	if opts.AutoMigrate {
		if err := Migrate(db); err != nil {
			return nil, err
		}
	} else if err := rejectIncompatibleSchema(db); err != nil {
		// Migrate checks ownership before changing the schema; apply the same
		// safeguard when migrations are disabled.
		return nil, err
	}
	if opts.RequireSchema {
		if err := CheckSchema(db); err != nil {
			return nil, err
		}
	}
	if opts.Seed {
		if err := Seed(db); err != nil {
			return nil, err
		}
	}
	initialized = true
	// Startup context must not become the default context of runtime queries.
	return db.WithContext(context.Background()), nil
}

func dialectorFor(driver, dsn string) (gorm.Dialector, error) {
	switch driver {
	case "sqlite":
		return sqlite.Open(dsn), nil
	case "postgres":
		return postgres.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

// Ping verifies the database is reachable. Used by the readiness probe.
func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func logLevel(quiet bool) gormlogger.LogLevel {
	if quiet {
		return gormlogger.Silent
	}
	return gormlogger.Warn
}

// ensureParentDir creates the parent directory for file backed SQLite databases.
func ensureParentDir(driver, dsn string) error {
	if driver != "sqlite" {
		return nil
	}
	if dsn == ":memory:" || strings.Contains(dsn, "mode=memory") {
		return nil
	}
	path := strings.TrimPrefix(dsn, "file:")
	if path == "" {
		return errors.New("sqlite DSN is empty")
	}
	return os.MkdirAll(filepath.Dir(path), 0o750)
}
