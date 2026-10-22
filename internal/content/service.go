// Package content owns MR1 authorization, transactions and private content workflows.
package content

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cotests/internal/artifact"
	"cotests/internal/db"
	"cotests/internal/importer"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrForbidden    = errors.New("access denied")
	ErrUnauthorized = errors.New("authentication required")
	ErrConflict     = errors.New("resource is not editable or request conflicts with current state")
)

type ValidationError = db.ValidationError

func invalid(message string) error { return ValidationError{Message: message} }

type Service struct {
	DB           *gorm.DB
	Store        artifact.Store
	ImportLimits importer.Limits
	Now          func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s *Service) database(ctx context.Context) *gorm.DB { return s.DB.WithContext(ctx) }
func requireUser(user *db.User) error {
	if user == nil {
		return ErrUnauthorized
	}
	return nil
}
func admin(user *db.User) bool { return user != nil && user.Role == db.RoleAdmin }

func (s *Service) ManageContest(ctx context.Context, actor *db.User, id uint) error {
	if err := requireUser(actor); err != nil {
		return err
	}
	var contest db.Contest
	if err := s.database(ctx).First(&contest, id).Error; err != nil {
		return err
	}
	if admin(actor) {
		return nil
	}
	var count int64
	if err := s.database(ctx).Model(&db.ContestMembership{}).Where("contest_id = ? AND user_id = ? AND role = ?", id, actor.ID, "organizer").Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrForbidden
	}
	return nil
}

func (s *Service) ManageProblem(ctx context.Context, actor *db.User, id uint) (*db.Problem, error) {
	if err := requireUser(actor); err != nil {
		return nil, err
	}
	problem, err := db.GetProblem(s.database(ctx), id)
	if err != nil {
		return nil, err
	}
	if !admin(actor) && problem.OwnerID != actor.ID {
		return nil, ErrForbidden
	}
	return problem, nil
}

func (s *Service) Revision(ctx context.Context, actor *db.User, id uint) (*db.ProblemRevision, error) {
	revision, err := db.GetProblemRevision(s.database(ctx), id)
	if err != nil {
		return nil, err
	}
	_, err = s.ManageProblem(ctx, actor, revision.ProblemID)
	return revision, err
}

func (s *Service) CanAuthor(ctx context.Context, actor *db.User) error {
	if err := requireUser(actor); err != nil {
		return err
	}
	if admin(actor) {
		return nil
	}
	var count int64
	if err := s.database(ctx).Model(&db.ContestMembership{}).Where("user_id = ? AND role = ?", actor.ID, "organizer").Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrForbidden
	}
	return nil
}

func (s *Service) SetMember(ctx context.Context, actor *db.User, contestID, userID uint, role string) error {
	return s.setMember(ctx, actor, contestID, userID, role, false)
}

func (s *Service) RemoveMember(ctx context.Context, actor *db.User, contestID, userID uint) error {
	return s.setMember(ctx, actor, contestID, userID, "", true)
}

func (s *Service) setMember(ctx context.Context, actor *db.User, contestID, userID uint, role string, remove bool) error {
	if err := s.ManageContest(ctx, actor, contestID); err != nil {
		return err
	}
	if role != "organizer" && role != "participant" && !remove {
		return invalid("invalid membership role")
	}
	return s.database(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize grants and revocations, including organizer demotion.
		if err := tx.Model(&db.Contest{}).Where("id = ?", contestID).UpdateColumn("id", contestID).Error; err != nil {
			return err
		}
		var existing db.ContestMembership
		err := tx.Where("contest_id = ? AND user_id = ?", contestID, userID).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if !admin(actor) && (role == "organizer" || existing.Role == "organizer") {
			return ErrForbidden
		}
		var user db.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}
		if remove {
			return tx.Where("contest_id = ? AND user_id = ?", contestID, userID).Delete(&db.ContestMembership{}).Error
		}
		member := db.ContestMembership{ContestID: contestID, UserID: userID, Role: role}
		return tx.Omit("Contest", "User").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "contest_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"role"})}).Create(&member).Error
	})
}

func (s *Service) Series(ctx context.Context, actor *db.User, id uint) (*db.Series, error) {
	series, err := db.GetSeries(s.database(ctx), id)
	if err != nil {
		return nil, err
	}
	if err := s.ManageContest(ctx, actor, series.ContestID); err != nil {
		return nil, err
	}
	return series, nil
}

func (s *Service) SaveSeries(ctx context.Context, actor *db.User, series *db.Series) error {
	if err := s.ManageContest(ctx, actor, series.ContestID); err != nil {
		return err
	}
	database := s.database(ctx)
	if series.ID == 0 {
		return db.CreateSeries(database, series)
	}
	return db.UpdateSeries(database, series)
}

func (s *Service) CanReadSeries(ctx context.Context, actor *db.User, series *db.Series) error {
	if err := s.ManageContest(ctx, actor, series.ContestID); err == nil {
		return nil
	} else if !errors.Is(err, ErrForbidden) && !errors.Is(err, ErrUnauthorized) {
		return err
	}
	if err := requireUser(actor); err != nil {
		return err
	}
	contest, err := db.GetContest(s.database(ctx), series.ContestID)
	if err != nil {
		return err
	}
	if contest.Visibility != db.ContestPublished {
		return ErrForbidden
	}
	now := s.now()
	if contest.StartAt != nil && now.Before(*contest.StartAt) {
		return ErrForbidden
	}
	release := series.ReleaseAt
	if release == nil {
		release = series.StartAt
	}
	if release != nil && now.Before(*release) {
		return ErrForbidden
	}
	var count int64
	if err := s.database(ctx).Model(&db.ContestMembership{}).Where("contest_id = ? AND user_id = ?", series.ContestID, actor.ID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrForbidden
	}
	return nil
}

// CanSubmit is the shared eligibility policy for MR2; no submission is created here.
func (s *Service) CanSubmit(ctx context.Context, actor *db.User, seriesID uint) error {
	if err := requireUser(actor); err != nil {
		return err
	}
	series, err := db.GetSeries(s.database(ctx), seriesID)
	if err != nil {
		return err
	}
	if err := s.CanReadSeries(ctx, actor, series); err != nil {
		return err
	}
	var count int64
	if err := s.database(ctx).Model(&db.ContestMembership{}).Where("contest_id = ? AND user_id = ? AND role = ?", series.ContestID, actor.ID, "participant").Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrForbidden
	}
	contest, err := db.GetContest(s.database(ctx), series.ContestID)
	if err != nil {
		return err
	}
	if contest.Visibility != db.ContestPublished {
		return ErrForbidden
	}
	now := s.now()
	for _, window := range [][2]*time.Time{{contest.StartAt, contest.EndAt}, {series.StartAt, series.EndAt}} {
		if window[0] != nil && now.Before(*window[0]) || window[1] != nil && !now.Before(*window[1]) {
			return ErrForbidden
		}
	}
	return nil
}

func (s *Service) Assign(ctx context.Context, actor *db.User, assignment *db.SeriesProblem) error {
	if _, err := s.Series(ctx, actor, assignment.SeriesID); err != nil {
		return err
	}
	revision, err := s.Revision(ctx, actor, assignment.PublishedRevisionID)
	if err != nil {
		return err
	}
	if revision.State != db.RevisionPublished {
		return ErrConflict
	}
	assignment.ProblemID = revision.ProblemID
	assignment.Code = strings.TrimSpace(assignment.Code)
	if assignment.Code == "" || len(assignment.Code) > 64 || assignment.Position < 0 {
		return invalid("invalid assignment code or position")
	}
	return s.database(ctx).Omit("Series", "Problem", "PublishedRevision").Create(assignment).Error
}

func (s *Service) ReorderSeries(ctx context.Context, actor *db.User, contestID uint, ids []uint) error {
	if err := s.ManageContest(ctx, actor, contestID); err != nil {
		return err
	}
	return s.reorder(ctx, contestID, ids, &db.Series{}, &db.Contest{}, "contest_id")
}

func (s *Service) ReorderProblems(ctx context.Context, actor *db.User, seriesID uint, ids []uint) error {
	if _, err := s.Series(ctx, actor, seriesID); err != nil {
		return err
	}
	return s.reorder(ctx, seriesID, ids, &db.SeriesProblem{}, &db.Series{}, "series_id")
}

// reorder requires the complete set and uses a disjoint positive range, retaining
// non-negative check constraints and uniqueness on both supported databases.
func (s *Service) reorder(ctx context.Context, parent uint, ids []uint, model, parentModel any, column string) error {
	return s.database(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(parentModel).Where("id = ?", parent).UpdateColumn("id", parent).Error; err != nil {
			return err
		}
		var rows []struct {
			ID       uint
			Position int
		}
		if err := tx.Model(model).Where(column+" = ?", parent).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return invalid("order must contain every resource exactly once")
		}
		seen := map[uint]bool{}
		max := 0
		for _, row := range rows {
			seen[row.ID] = true
			if row.Position > max {
				max = row.Position
			}
		}
		if max > 1<<30 || len(ids) > 10000 {
			return invalid("order is too large")
		}
		for i, id := range ids {
			if !seen[id] {
				return invalid("invalid or duplicate order ID")
			}
			delete(seen, id)
			if err := tx.Model(model).Where("id = ?", id).Update("position", max+1+i).Error; err != nil {
				return err
			}
		}
		for i, id := range ids {
			if err := tx.Model(model).Where("id = ?", id).Update("position", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type PublicSample struct {
	Position         int  `json:"position"`
	InputArtifactID  uint `json:"input_artifact_id"`
	OutputArtifactID uint `json:"output_artifact_id"`
}
type Statement struct {
	AssignmentID    uint           `json:"assignment_id"`
	RevisionID      uint           `json:"revision_id"`
	Title           string         `json:"title"`
	Code            string         `json:"code"`
	Statement       string         `json:"statement"`
	PDFArtifactID   *uint          `json:"pdf_artifact_id"`
	TimeLimitMillis *int64         `json:"time_limit_millis"`
	MemoryLimitKiB  *int64         `json:"memory_limit_kib"`
	Languages       string         `json:"language_metadata"`
	Samples         []PublicSample `json:"samples"`
}

func (s *Service) Statement(ctx context.Context, actor *db.User, id uint) (*Statement, error) {
	var assignment db.SeriesProblem
	if err := s.database(ctx).First(&assignment, id).Error; err != nil {
		return nil, err
	}
	series, err := db.GetSeries(s.database(ctx), assignment.SeriesID)
	if err != nil {
		return nil, err
	}
	if err := s.CanReadSeries(ctx, actor, series); err != nil {
		return nil, err
	}
	revision, err := db.GetProblemRevision(s.database(ctx), assignment.PublishedRevisionID)
	if err != nil {
		return nil, err
	}
	if revision.State != db.RevisionPublished {
		return nil, ErrForbidden
	}
	problem, err := db.GetProblem(s.database(ctx), assignment.ProblemID)
	if err != nil {
		return nil, err
	}
	result := &Statement{AssignmentID: id, RevisionID: revision.ID, Title: problem.Title, Code: assignment.Code, Statement: revision.Statement, PDFArtifactID: revision.PDFArtifactID, TimeLimitMillis: revision.TimeLimitMillis, MemoryLimitKiB: revision.MemoryLimitKiB, Languages: revision.LanguageMetadata, Samples: []PublicSample{}}
	var tests []db.TestCase
	if err := s.database(ctx).Where("problem_revision_id = ? AND is_sample = ?", revision.ID, true).Order("position").Find(&tests).Error; err != nil {
		return nil, err
	}
	for _, test := range tests {
		result.Samples = append(result.Samples, PublicSample{test.Position, test.InputArtifactID, test.OutputArtifactID})
	}
	return result, nil
}

func (s *Service) Artifact(ctx context.Context, actor *db.User, id uint) (*db.Artifact, error) {
	if err := requireUser(actor); err != nil {
		return nil, err
	}
	var item db.Artifact
	if err := s.database(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	if admin(actor) {
		return &item, nil
	}
	// Ownership of a storage record alone is insufficient: it must still be
	// attached to content for which the user has authority.
	var revisions []db.ProblemRevision
	if err := s.database(ctx).Where("pdf_artifact_id = ? OR id IN (SELECT problem_revision_id FROM test_cases WHERE input_artifact_id = ? OR output_artifact_id = ?)", id, id, id).Find(&revisions).Error; err != nil {
		return nil, err
	}
	for _, revision := range revisions {
		if _, err := s.ManageProblem(ctx, actor, revision.ProblemID); err == nil {
			return &item, nil
		} else if !errors.Is(err, ErrForbidden) {
			return nil, err
		}
		if revision.State != db.RevisionPublished {
			continue
		}
		allowed := revision.PDFArtifactID != nil && *revision.PDFArtifactID == id
		if !allowed {
			var count int64
			if err := s.database(ctx).Model(&db.TestCase{}).Where("problem_revision_id = ? AND is_sample = ? AND (input_artifact_id = ? OR output_artifact_id = ?)", revision.ID, true, id, id).Count(&count).Error; err != nil {
				return nil, err
			}
			allowed = count > 0
		}
		if !allowed {
			continue
		}
		var assignments []db.SeriesProblem
		if err := s.database(ctx).Where("published_revision_id = ?", revision.ID).Find(&assignments).Error; err != nil {
			return nil, err
		}
		for _, assignment := range assignments {
			series, err := db.GetSeries(s.database(ctx), assignment.SeriesID)
			if err != nil {
				return nil, err
			}
			if err := s.CanReadSeries(ctx, actor, series); err == nil {
				return &item, nil
			} else if !errors.Is(err, ErrForbidden) {
				return nil, err
			}
		}
	}
	return nil, ErrForbidden
}

func conflictDraft(err error) error {
	if errors.Is(err, db.ErrNotDraft) {
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	return err
}
