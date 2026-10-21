package db

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const schemaMigrationsTable = "schema_migrations"

type schemaMigration struct {
	Version   uint      `gorm:"primaryKey;autoIncrement:false"`
	AppliedAt time.Time `gorm:"not null"`
}

func (schemaMigration) TableName() string { return schemaMigrationsTable }

type migration struct {
	version uint
	apply   func(*gorm.DB) error
}

var migrations = []migration{
	{version: 1, apply: createInitialSchema},
	{version: 2, apply: removeLegacySessionTokens},
	{version: 3, apply: addSessionIntegrity},
	{version: 4, apply: requireSessionFields},
	{version: 5, apply: createContentSchema},
	{version: 6, apply: createAccessSchema},
	{version: 7, apply: scopeAssignmentCodes},
}

// Older content schemas made display codes globally unique. Codes belong to a
// series, so two rounds must both be able to use "A" without sharing content.
func scopeAssignmentCodes(database *gorm.DB) error {
	if database.Migrator().HasIndex(&SeriesProblem{}, "idx_series_problem_code") {
		if err := database.Migrator().DropIndex(&SeriesProblem{}, "idx_series_problem_code"); err != nil {
			return err
		}
	}
	return database.Migrator().CreateIndex(&SeriesProblem{}, "idx_series_problem_code")
}

func createAccessSchema(database *gorm.DB) error {
	for _, column := range []string{"StartAt", "EndAt", "ReleaseAt"} {
		if !database.Migrator().HasColumn(&Series{}, column) {
			if err := database.Migrator().AddColumn(&Series{}, column); err != nil {
				return err
			}
		}
	}
	for _, model := range []any{&ContestMembership{}, &ImportPreview{}} {
		if !database.Migrator().HasTable(model) {
			if err := database.Migrator().CreateTable(model); err != nil {
				return err
			}
		}
	}
	return nil
}

func createContentSchema(database *gorm.DB) error {
	for _, model := range []any{&Artifact{}, &Problem{}, &ProblemRevision{}, &migrationSeriesProblem{}, &TestCase{}} {
		if database.Migrator().HasTable(model) {
			continue
		}
		if err := database.Migrator().CreateTable(model); err != nil {
			return err
		}
	}
	return nil
}

// Freeze the original migration-5 assignment index. Migration 7 repairs its
// scope even when upgrading an existing installation that already applied 5.
type migrationSeriesProblem struct {
	ID                  uint   `gorm:"primaryKey"`
	SeriesID            uint   `gorm:"not null;uniqueIndex:idx_series_problem_position,priority:1"`
	ProblemID           uint   `gorm:"not null;index"`
	PublishedRevisionID uint   `gorm:"not null"`
	Code                string `gorm:"not null;check:code <> '';uniqueIndex:idx_series_problem_code,priority:1"`
	Position            int    `gorm:"not null;check:position >= 0;uniqueIndex:idx_series_problem_position,priority:2"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Series              Series          `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Problem             Problem         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	PublishedRevision   ProblemRevision `gorm:"foreignKey:PublishedRevisionID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

func (migrationSeriesProblem) TableName() string { return "series_problems" }

// Migrate applies each immutable schema migration once and records its version.
// It also upgrades databases created before schema_migrations existed.
func Migrate(database *gorm.DB) error {
	if !database.Migrator().HasTable(&schemaMigration{}) {
		if err := database.Migrator().CreateTable(&schemaMigration{}); err != nil {
			return fmt.Errorf("create migration table: %w", err)
		}
	}

	return database.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(0x435445535453)).Error; err != nil {
				return fmt.Errorf("lock migrations: %w", err)
			}
		}

		for _, migration := range migrations {
			var applied schemaMigration
			err := tx.First(&applied, "version = ?", migration.version).Error
			if err == nil {
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("read migration %d: %w", migration.version, err)
			}

			if err := migration.apply(tx); err != nil {
				return fmt.Errorf("apply migration %d: %w", migration.version, err)
			}
			if err := tx.Create(&schemaMigration{Version: migration.version, AppliedAt: time.Now().UTC()}).Error; err != nil {
				return fmt.Errorf("record migration %d: %w", migration.version, err)
			}
		}
		return nil
	})
}

func createInitialSchema(database *gorm.DB) error {
	for _, model := range []any{&migrationUser{}, &migrationSession{}, &migrationContest{}, &migrationSeries{}} {
		if database.Migrator().HasTable(model) {
			continue
		}
		if err := database.Migrator().CreateTable(model); err != nil {
			return err
		}
	}
	return nil
}

func removeLegacySessionTokens(database *gorm.DB) error {
	if !database.Migrator().HasTable(&Session{}) || !database.Migrator().HasColumn(&Session{}, "token") {
		return nil
	}

	// A raw session token is a credential. Invalidate all old sessions before
	// removing the column; their values must never be copied into the new schema.
	if err := database.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Session{}).Error; err != nil {
		return fmt.Errorf("invalidate legacy sessions: %w", err)
	}
	if !database.Migrator().HasColumn(&Session{}, "token_hash") {
		if err := database.Migrator().AddColumn(&Session{}, "TokenHash"); err != nil {
			return fmt.Errorf("add token hash: %w", err)
		}
	}
	if err := database.Migrator().DropColumn(&Session{}, "token"); err != nil {
		return fmt.Errorf("drop raw token: %w", err)
	}
	return nil
}

func addSessionIntegrity(database *gorm.DB) error {
	if !database.Migrator().HasConstraint(&Session{}, "User") {
		if err := rejectOrphanedSessions(database); err != nil {
			return err
		}
		if err := database.Migrator().CreateConstraint(&Session{}, "User"); err != nil {
			return fmt.Errorf("create session user foreign key: %w", err)
		}
	}
	// SQLite rebuilds a table when adding a foreign key, which can discard an
	// index created earlier in this migration. Create the index afterwards.
	if !database.Migrator().HasIndex(&Session{}, "ExpiresAt") {
		if err := database.Migrator().CreateIndex(&Session{}, "ExpiresAt"); err != nil {
			return fmt.Errorf("create expiry index: %w", err)
		}
	}
	return nil
}

func requireSessionFields(database *gorm.DB) error {
	var invalid int64
	if err := database.Table("sessions").
		Where("token_hash IS NULL OR token_hash = '' OR expires_at IS NULL").
		Count(&invalid).Error; err != nil {
		return fmt.Errorf("check incomplete sessions: %w", err)
	}
	if invalid > 0 {
		return fmt.Errorf("cannot require session fields: %d incomplete sessions must be removed first", invalid)
	}
	if err := database.Migrator().AlterColumn(&Session{}, "TokenHash"); err != nil {
		return fmt.Errorf("require token hash: %w", err)
	}
	if err := database.Migrator().AlterColumn(&Session{}, "ExpiresAt"); err != nil {
		return fmt.Errorf("require session expiry: %w", err)
	}
	// SQLite rebuilds tables for ALTER COLUMN, so restore the constraint and
	// index after the final rebuild.
	return addSessionIntegrity(database)
}

func rejectOrphanedSessions(database *gorm.DB) error {
	var count int64
	if err := database.Table("sessions").
		Where("NOT EXISTS (SELECT 1 FROM users WHERE users.id = sessions.user_id)").
		Count(&count).Error; err != nil {
		return fmt.Errorf("check orphaned sessions: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot add session user foreign key: %d orphaned sessions must be removed first", count)
	}
	return nil
}

// These types freeze the schema created by migration 1. Future model changes
// must be introduced by a new migration rather than changing this snapshot.
type migrationUser struct {
	ID           uint   `gorm:"primaryKey"`
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	Name         string
	Role         string `gorm:"not null;default:user;index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (migrationUser) TableName() string { return "users" }

type migrationSession struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index;not null"`
	TokenHash string `gorm:"uniqueIndex"`
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (migrationSession) TableName() string { return "sessions" }

type migrationContest struct {
	ID          uint   `gorm:"primaryKey"`
	Title       string `gorm:"not null;check:title <> ''"`
	Description string
	Visibility  string     `gorm:"not null;default:draft;check:visibility IN ('draft','published');index"`
	StartAt     *time.Time `gorm:"check:contest_dates,end_at IS NULL OR start_at IS NULL OR end_at > start_at"`
	EndAt       *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (migrationContest) TableName() string { return "contests" }

type migrationSeries struct {
	ID        uint   `gorm:"primaryKey"`
	ContestID uint   `gorm:"not null;index;uniqueIndex:idx_series_contest_position,priority:1"`
	Title     string `gorm:"not null;check:title <> ''"`
	Position  int    `gorm:"not null;check:position >= 0;uniqueIndex:idx_series_contest_position,priority:2"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Contest   migrationContest `gorm:"foreignKey:ContestID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func (migrationSeries) TableName() string { return "series" }
