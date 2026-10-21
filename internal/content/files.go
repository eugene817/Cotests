package content

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"cotests/internal/artifact"
	"cotests/internal/db"
	"cotests/internal/importer"
	"gorm.io/gorm"
)

type saveArtifact func(uint, string, string, io.Reader) (*db.Artifact, error)

// fileTransaction compensates filesystem writes whenever the database transaction
// fails. A process crash can still leave an orphan; GC handles that after grace.
func (s *Service) fileTransaction(ctx context.Context, store artifact.Store, fn func(*gorm.DB, saveArtifact) error) error {
	var created []*db.Artifact
	err := s.database(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx, func(owner uint, name, kind string, source io.Reader) (*db.Artifact, error) {
			item, err := store.Save(tx, owner, name, kind, source)
			if err == nil {
				created = append(created, item)
			}
			return item, err
		})
	})
	if err != nil {
		for _, item := range created {
			_ = store.Remove(item.StorageKey)
		}
	}
	return err
}

type TestInput struct {
	Position                int    `json:"position"`
	IsSample                bool   `json:"is_sample"`
	Points                  *int64 `json:"points"`
	TimeLimitOverrideMillis *int64 `json:"time_limit_override_millis"`
	Input                   string `json:"input"`
	Output                  string `json:"output"`
}

func (s *Service) SaveTest(ctx context.Context, actor *db.User, revisionID, testID uint, input TestInput) (*db.TestCase, error) {
	if _, err := s.Revision(ctx, actor, revisionID); err != nil {
		return nil, err
	}
	if input.Position < 0 || input.Points != nil && *input.Points < 0 || input.TimeLimitOverrideMillis != nil && *input.TimeLimitOverrideMillis <= 0 {
		return nil, invalid("invalid test metadata")
	}
	result := &db.TestCase{ID: testID, ProblemRevisionID: revisionID, Position: input.Position, IsSample: input.IsSample, Points: input.Points, TimeLimitOverrideMillis: input.TimeLimitOverrideMillis}
	err := s.fileTransaction(ctx, s.Store, func(tx *gorm.DB, save saveArtifact) error {
		if err := db.LockDraft(tx, revisionID); err != nil {
			return conflictDraft(err)
		}
		if testID != 0 {
			var existing db.TestCase
			if err := tx.Where("id = ? AND problem_revision_id = ?", testID, revisionID).First(&existing).Error; err != nil {
				return err
			}
		}
		in, err := save(actor.ID, "input.txt", "text/plain", strings.NewReader(input.Input))
		if err != nil {
			return err
		}
		out, err := save(actor.ID, "output.txt", "text/plain", strings.NewReader(input.Output))
		if err != nil {
			return err
		}
		result.InputArtifactID = in.ID
		result.OutputArtifactID = out.ID
		if testID == 0 {
			return tx.Omit("Revision", "InputArtifact", "OutputArtifact").Create(result).Error
		}
		return tx.Model(&db.TestCase{}).Where("id = ?", testID).Updates(map[string]any{"position": input.Position, "is_sample": input.IsSample, "points": input.Points, "time_limit_override_millis": input.TimeLimitOverrideMillis, "input_artifact_id": in.ID, "output_artifact_id": out.ID}).Error
	})
	return result, err
}

func (s *Service) DeleteTest(ctx context.Context, actor *db.User, revisionID, testID uint) error {
	if _, err := s.Revision(ctx, actor, revisionID); err != nil {
		return err
	}
	return s.database(ctx).Transaction(func(tx *gorm.DB) error {
		if err := db.LockDraft(tx, revisionID); err != nil {
			return conflictDraft(err)
		}
		result := tx.Where("id = ? AND problem_revision_id = ?", testID, revisionID).Delete(&db.TestCase{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *Service) UploadPDF(ctx context.Context, actor *db.User, revisionID uint, source io.Reader) (*db.Artifact, error) {
	if _, err := s.Revision(ctx, actor, revisionID); err != nil {
		return nil, err
	}
	header := make([]byte, 5)
	if _, err := io.ReadFull(source, header); err != nil || string(header) != "%PDF-" {
		return nil, invalid("a PDF file is required")
	}
	var item *db.Artifact
	err := s.fileTransaction(ctx, s.Store, func(tx *gorm.DB, save saveArtifact) error {
		if err := db.LockDraft(tx, revisionID); err != nil {
			return conflictDraft(err)
		}
		var err error
		item, err = save(actor.ID, "statement.pdf", "application/pdf", io.MultiReader(bytes.NewReader(header), source))
		if err != nil {
			return err
		}
		return tx.Model(&db.ProblemRevision{}).Where("id = ?", revisionID).Update("pdf_artifact_id", item.ID).Error
	})
	return item, err
}

type Preview struct {
	ID        uint          `json:"id"`
	ExpiresAt time.Time     `json:"expires_at"`
	Tests     []PreviewTest `json:"tests"`
}
type PreviewTest struct {
	Name        string `json:"name"`
	InputBytes  int    `json:"input_bytes"`
	OutputBytes int    `json:"output_bytes"`
}

func (s *Service) PreviewZIP(ctx context.Context, actor *db.User, revisionID uint, source io.Reader) (*Preview, error) {
	revision, err := s.Revision(ctx, actor, revisionID)
	if err != nil {
		return nil, err
	}
	if revision.State != db.RevisionDraft {
		return nil, ErrConflict
	}
	raw, err := io.ReadAll(io.LimitReader(source, s.ImportLimits.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	pairs, err := importer.ReadTestZIP(bytes.NewReader(raw), s.ImportLimits)
	if err != nil {
		return nil, invalid(err.Error())
	}
	if len(pairs) == 0 {
		return nil, invalid("archive must contain tests")
	}
	for _, pair := range pairs {
		if int64(len(pair.Input)) > s.Store.MaxBytes || int64(len(pair.Output)) > s.Store.MaxBytes {
			return nil, invalid("test exceeds artifact size limit")
		}
	}
	preview := db.ImportPreview{OwnerID: actor.ID, RevisionID: revisionID, ExpiresAt: s.now().Add(time.Hour)}
	// ZIP storage uses its own bounded limit, not the individual test limit.
	store := s.Store
	store.MaxBytes = s.ImportLimits.MaxBytes
	err = s.fileTransaction(ctx, store, func(tx *gorm.DB, save saveArtifact) error {
		if err := db.LockDraft(tx, revisionID); err != nil {
			return conflictDraft(err)
		}
		item, err := save(actor.ID, "tests.zip", "application/zip", bytes.NewReader(raw))
		if err != nil {
			return err
		}
		preview.ArtifactID = item.ID
		return tx.Omit("Owner", "Revision", "Artifact").Create(&preview).Error
	})
	if err != nil {
		return nil, err
	}
	result := &Preview{ID: preview.ID, ExpiresAt: preview.ExpiresAt, Tests: []PreviewTest{}}
	for _, pair := range pairs {
		result.Tests = append(result.Tests, PreviewTest{pair.Name, len(pair.Input), len(pair.Output)})
	}
	return result, nil
}

func (s *Service) CommitZIP(ctx context.Context, actor *db.User, revisionID, previewID uint) error {
	if _, err := s.Revision(ctx, actor, revisionID); err != nil {
		return err
	}
	return s.fileTransaction(ctx, s.Store, func(tx *gorm.DB, save saveArtifact) error {
		if err := db.LockDraft(tx, revisionID); err != nil {
			return conflictDraft(err)
		}
		var preview db.ImportPreview
		if err := tx.Where("id = ? AND owner_id = ? AND revision_id = ?", previewID, actor.ID, revisionID).First(&preview).Error; err != nil {
			return err
		}
		if preview.CommittedAt != nil {
			return nil
		}
		if !s.now().Before(preview.ExpiresAt) {
			return ErrConflict
		}
		var item db.Artifact
		if err := tx.First(&item, preview.ArtifactID).Error; err != nil {
			return err
		}
		file, err := s.Store.Open(&item)
		if err != nil {
			return err
		}
		defer file.Close()
		pairs, err := importer.ReadTestZIP(file, s.ImportLimits)
		if err != nil {
			return invalid(err.Error())
		}
		var max int
		if err := tx.Model(&db.TestCase{}).Select("COALESCE(MAX(position), -1)").Where("problem_revision_id = ?", revisionID).Scan(&max).Error; err != nil {
			return err
		}
		if max > 1<<30 {
			return invalid("test positions exceed limit")
		}
		for i, pair := range pairs {
			in, err := save(actor.ID, pair.Name+".in", "text/plain", bytes.NewReader(pair.Input))
			if err != nil {
				return err
			}
			out, err := save(actor.ID, pair.Name+".out", "text/plain", bytes.NewReader(pair.Output))
			if err != nil {
				return err
			}
			test := db.TestCase{ProblemRevisionID: revisionID, Position: max + 1 + i, InputArtifactID: in.ID, OutputArtifactID: out.ID}
			if err := tx.Omit("Revision", "InputArtifact", "OutputArtifact").Create(&test).Error; err != nil {
				return err
			}
		}
		return tx.Model(&preview).Update("committed_at", s.now()).Error
	})
}

// Collect removes expired preview references and old unreferenced generated files.
// The grace period protects a file renamed just before its DB metadata commits.
func (s *Service) Collect(ctx context.Context, actor *db.User) (int, error) {
	if !admin(actor) {
		return 0, ErrForbidden
	}
	cutoff := s.now().Add(-time.Hour)
	removed := 0
	var unused []db.Artifact
	err := s.database(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at < ?", cutoff).Delete(&db.ImportPreview{}).Error; err != nil {
			return err
		}
		if err := tx.Where(`created_at < ?
			AND NOT EXISTS (SELECT 1 FROM problem_revisions WHERE pdf_artifact_id = artifacts.id)
			AND NOT EXISTS (SELECT 1 FROM test_cases WHERE input_artifact_id = artifacts.id OR output_artifact_id = artifacts.id)
			AND NOT EXISTS (SELECT 1 FROM import_previews WHERE artifact_id = artifacts.id)`, cutoff).Find(&unused).Error; err != nil {
			return err
		}
		for _, item := range unused {
			if err := tx.Delete(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	// Unlink only after metadata commits. A failed GC transaction can never
	// roll back metadata after its files have already been removed.
	for _, item := range unused {
		if err := s.Store.Remove(item.StorageKey); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, fmt.Errorf("remove unused artifact: %w", err)
		}
		removed++
	}
	staging, err := s.Store.CollectTemporary(cutoff)
	removed += staging
	if err != nil {
		return removed, err
	}
	entries, err := os.ReadDir(s.Store.Root)
	if errors.Is(err, os.ErrNotExist) {
		return removed, nil
	}
	if err != nil {
		return removed, err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		name := entry.Name()
		if len(name) != 64 {
			continue
		}
		if _, err := hex.DecodeString(name); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, err
		}
		if !info.Mode().IsRegular() || !info.ModTime().Before(cutoff) {
			continue
		}
		var count int64
		if err := s.database(ctx).Model(&db.Artifact{}).Where("storage_key = ?", name).Count(&count).Error; err != nil {
			return removed, err
		}
		if count != 0 {
			continue
		}
		if err := s.Store.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, fmt.Errorf("remove orphan: %w", err)
		}
		removed++
	}
	return removed, nil
}
