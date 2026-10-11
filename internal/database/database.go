package database

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/awang-karisma/life-dashboard-hub/internal/config"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

type DB struct {
	*gorm.DB
	Dialect Dialect
}

func Open(cfg *config.Config) (*DB, error) {
	var dialector gorm.Dialector
	var dialect Dialect

	if isPostgres(cfg.DatabaseURL) {
		dialector = postgres.Open(cfg.DatabaseURL)
		dialect = DialectPostgres
	} else {
		dialect = DialectSQLite
		if err := os.MkdirAll(cfg.DataDir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create data dir: %w", err)
		}
		dbPath := filepath.Join(cfg.DataDir, "life-dashboard.db")
		// WAL mode, 5000ms busy timeout, foreign keys enabled
		dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", dbPath)
		dialector = sqlite.Open(dsn)
	}

	gormLogLevel := logger.Warn
	if strings.EqualFold(cfg.LogLevel, "debug") {
		gormLogLevel = logger.Info
	} else if strings.EqualFold(cfg.LogLevel, "error") {
		gormLogLevel = logger.Error
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	gdb, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Auto-migrate all models
	err = gdb.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.Payload{},
		&models.Record{},
		&models.Tombstone{},
		&models.Bucket{},
		&models.Family{},
		&models.FamilyMembership{},
		&models.Invitation{},
		&models.Session{},
	)
	if err != nil {
		return nil, fmt.Errorf("auto-migration failed: %w", err)
	}

	slog.Info("Database initialized successfully", "dialect", dialect)
	return &DB{DB: gdb, Dialect: dialect}, nil
}

// OpenTestDB opens an in-memory or temporary database for testing
func OpenTestDB(databaseURL string) (*DB, error) {
	if isPostgres(databaseURL) {
		return Open(&config.Config{
			DatabaseURL: databaseURL,
			LogLevel:    "error",
		})
	}

	// In-memory SQLite for testing
	dsn := ":memory:?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, err
	}

	if err := gdb.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.Payload{},
		&models.Record{},
		&models.Tombstone{},
		&models.Bucket{},
		&models.Family{},
		&models.FamilyMembership{},
		&models.Invitation{},
		&models.Session{},
	); err != nil {
		return nil, err
	}

	return &DB{DB: gdb, Dialect: DialectSQLite}, nil
}

func isPostgres(url string) bool {
	u := strings.ToLower(url)
	return strings.HasPrefix(u, "postgres://") || strings.HasPrefix(u, "postgresql://")
}

var ErrNotFound = errors.New("record not found")
