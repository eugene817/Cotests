package db

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func Open(dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	if isPostgres(dsn) {
		dialector = postgres.Open(dsn)
	} else {
		dialector = sqlite.Open(dsn)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.New(log.Default(), gormlogger.Config{
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("db underlying: %w", err)
	}
	if !isPostgres(dsn) {
		// SQLite permits only one writer, so serialise access through one connection.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxIdleTime(5 * time.Minute)
		if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
			return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
		}
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return db, nil
}

func isPostgres(dsn string) bool {
	return strings.HasPrefix(dsn, "postgres://") ||
		strings.HasPrefix(dsn, "postgresql://") ||
		strings.HasPrefix(dsn, "host=")
}
