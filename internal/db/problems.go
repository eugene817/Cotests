package db

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

func CreateProblem(database *gorm.DB, problem *Problem, revision *ProblemRevision) error {
	if err := validateProblem(problem); err != nil {
		return ValidationError{err.Error()}
	}
	if err := validateRevision(revision); err != nil {
		return ValidationError{err.Error()}
	}
	return database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(problem).Error; err != nil {
			return err
		}
		revision.ProblemID = problem.ID
		revision.Number = 1
		revision.State = RevisionDraft
		return tx.Create(revision).Error
	})
}

func GetProblem(database *gorm.DB, id uint) (*Problem, error) {
	var problem Problem
	if err := database.First(&problem, id).Error; err != nil {
		return nil, err
	}
	return &problem, nil
}

func ListProblems(database *gorm.DB) ([]Problem, error) {
	var problems []Problem
	if err := database.Order("id DESC").Find(&problems).Error; err != nil {
		return nil, err
	}
	return problems, nil
}

func GetProblemRevision(database *gorm.DB, id uint) (*ProblemRevision, error) {
	var revision ProblemRevision
	if err := database.Preload("PDFArtifact").First(&revision, id).Error; err != nil {
		return nil, err
	}
	return &revision, nil
}

func ListProblemRevisions(database *gorm.DB, problemID uint) ([]ProblemRevision, error) {
	var revisions []ProblemRevision
	if err := database.Where("problem_id = ?", problemID).Order("number DESC").Find(&revisions).Error; err != nil {
		return nil, err
	}
	return revisions, nil
}

func UpdateDraftRevision(database *gorm.DB, revision *ProblemRevision) error {
	if revision.ID == 0 || revision.State != RevisionDraft {
		return ErrNotDraft
	}
	if err := validateRevision(revision); err != nil {
		return ValidationError{err.Error()}
	}
	return database.Transaction(func(tx *gorm.DB) error {
		if err := LockDraft(tx, revision.ID); err != nil {
			return err
		}
		result := tx.Model(&ProblemRevision{}).Where("id = ? AND state = ?", revision.ID, RevisionDraft).Updates(map[string]any{
			"statement": revision.Statement, "pdf_artifact_id": revision.PDFArtifactID,
			"time_limit_millis": revision.TimeLimitMillis, "memory_limit_kib": revision.MemoryLimitKiB,
			"language_metadata": revision.LanguageMetadata, "checker_metadata": revision.CheckerMetadata,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotDraft
		}
		return nil
	})
}

func PublishRevision(database *gorm.DB, revisionID uint, now time.Time) error {
	return database.Transaction(func(tx *gorm.DB) error {
		if err := LockDraft(tx, revisionID); err != nil {
			return err
		}
		revision, err := GetProblemRevision(tx, revisionID)
		if err != nil {
			return err
		}
		if err := validateRevision(revision); err != nil {
			return ValidationError{err.Error()}
		}
		if revision.Statement == "" && revision.PDFArtifactID == nil {
			return ValidationError{"statement or PDF is required for publication"}
		}
		result := tx.Model(&ProblemRevision{}).Where("id = ? AND state = ?", revisionID, RevisionDraft).Updates(map[string]any{
			"state": RevisionPublished, "published_at": now.UTC(),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotDraft
		}
		return nil
	})
}

// LockDraft serializes all content changes and publication on the revision row.
// A conditional write works with both PostgreSQL row locks and SQLite writers.
func LockDraft(tx *gorm.DB, id uint) error {
	result := tx.Model(&ProblemRevision{}).Where("id = ? AND state = ?", id, RevisionDraft).UpdateColumn("state", RevisionDraft)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrNotDraft
	}
	return nil
}

func CreateDraftRevision(database *gorm.DB, problemID uint) (*ProblemRevision, error) {
	var revision ProblemRevision
	err := database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Problem{}).Where("id = ?", problemID).UpdateColumn("id", problemID).Error; err != nil {
			return err
		}
		var latest ProblemRevision
		if err := tx.Where("problem_id = ?", problemID).Order("number DESC").First(&latest).Error; err != nil {
			return err
		}
		// Clone a coherent revision/test snapshot while concurrent edits wait.
		if err := tx.Model(&ProblemRevision{}).Where("id = ?", latest.ID).UpdateColumn("id", latest.ID).Error; err != nil {
			return err
		}
		if err := tx.First(&latest, latest.ID).Error; err != nil {
			return err
		}
		revision = ProblemRevision{ProblemID: problemID, Number: latest.Number + 1, State: RevisionDraft, Statement: latest.Statement, PDFArtifactID: latest.PDFArtifactID, TimeLimitMillis: latest.TimeLimitMillis, MemoryLimitKiB: latest.MemoryLimitKiB, LanguageMetadata: latest.LanguageMetadata, CheckerMetadata: latest.CheckerMetadata}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		var tests []TestCase
		if err := tx.Where("problem_revision_id = ?", latest.ID).Find(&tests).Error; err != nil {
			return err
		}
		for _, test := range tests {
			test.ID = 0
			test.ProblemRevisionID = revision.ID
			if err := tx.Omit("Revision", "InputArtifact", "OutputArtifact").Create(&test).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &revision, nil
}

func ListTestCases(database *gorm.DB, revisionID uint) ([]TestCase, error) {
	var tests []TestCase
	if err := database.Where("problem_revision_id = ?", revisionID).Order("position ASC").Find(&tests).Error; err != nil {
		return nil, err
	}
	return tests, nil
}

func CreateTestCase(database *gorm.DB, test *TestCase) error {
	if test.ProblemRevisionID == 0 || test.InputArtifactID == 0 || test.OutputArtifactID == 0 || test.Position < 0 {
		return ValidationError{"invalid test case"}
	}
	return database.Transaction(func(tx *gorm.DB) error {
		if err := LockDraft(tx, test.ProblemRevisionID); err != nil {
			return err
		}
		return tx.Omit("Revision", "InputArtifact", "OutputArtifact").Create(test).Error
	})
}

func validateProblem(problem *Problem) error {
	problem.Title = strings.TrimSpace(problem.Title)
	problem.Code = strings.TrimSpace(problem.Code)
	if problem.OwnerID == 0 || problem.Title == "" || problem.Code == "" {
		return ValidationError{"problem owner, title and code are required"}
	}
	if len(problem.Title) > 300 || len(problem.Code) > 64 {
		return ValidationError{"problem title or code is too long"}
	}
	if problem.Kind == "" {
		problem.Kind = "programming"
	}
	if problem.Kind != "programming" {
		return ValidationError{"invalid problem kind"}
	}
	return nil
}

func validateRevision(revision *ProblemRevision) error {
	revision.Statement = strings.TrimSpace(revision.Statement)
	revision.LanguageMetadata = strings.TrimSpace(revision.LanguageMetadata)
	revision.CheckerMetadata = strings.TrimSpace(revision.CheckerMetadata)
	if revision.LanguageMetadata == "" {
		revision.LanguageMetadata = "[]"
	}
	var languages []string
	if err := json.Unmarshal([]byte(revision.LanguageMetadata), &languages); err != nil || languages == nil || len(languages) > 100 {
		return ValidationError{"languages must be an array of at most 100 identifiers"}
	}
	seen := make(map[string]bool)
	for _, language := range languages {
		if strings.TrimSpace(language) != language || language == "" || len(language) > 100 || seen[language] {
			return ValidationError{"invalid or duplicate language identifier"}
		}
		seen[language] = true
	}
	if len(revision.Statement) > 512<<10 || len(revision.CheckerMetadata) > 200 {
		return ValidationError{"revision metadata is too large"}
	}
	if revision.TimeLimitMillis != nil && *revision.TimeLimitMillis <= 0 || revision.MemoryLimitKiB != nil && *revision.MemoryLimitKiB <= 0 {
		return ValidationError{"limits must be positive"}
	}
	return nil
}

func IsNotFound(err error) bool { return err != nil && (err == gorm.ErrRecordNotFound) }
