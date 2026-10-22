package db

import (
	"cotests/internal/auth"
	"time"
)

type User struct {
	ID           uint   `gorm:"primaryKey"`
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null" json:"-"`
	Name         string
	Role         string `gorm:"not null;default:user;index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	TokenHash string    `gorm:"uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time `gorm:"index;not null"`
	CreatedAt time.Time
	User      User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Contest struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Title       string     `gorm:"not null;check:title <> ''" json:"title"`
	Description string     `json:"description"`
	Visibility  string     `gorm:"not null;default:draft;check:visibility IN ('draft','published');index" json:"visibility"`
	StartAt     *time.Time `gorm:"check:contest_dates,end_at IS NULL OR start_at IS NULL OR end_at > start_at" json:"start_at"`
	EndAt       *time.Time `json:"end_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Series      []Series   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

type Series struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	ContestID uint       `gorm:"not null;index;uniqueIndex:idx_series_contest_position,priority:1" json:"contest_id"`
	Title     string     `gorm:"not null;check:title <> ''" json:"title"`
	Position  int        `gorm:"not null;check:position >= 0;uniqueIndex:idx_series_contest_position,priority:2" json:"position"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Contest   Contest    `json:"-"`
	StartAt   *time.Time `json:"start_at"`
	EndAt     *time.Time `json:"end_at"`
	ReleaseAt *time.Time `json:"release_at"`
}

type ContestMembership struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	ContestID uint    `gorm:"not null;uniqueIndex:idx_membership,priority:1" json:"contest_id"`
	UserID    uint    `gorm:"not null;uniqueIndex:idx_membership,priority:2" json:"user_id"`
	Role      string  `gorm:"not null;check:role IN ('organizer','participant')" json:"role"`
	Contest   Contest `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	User      User    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

// ImportPreview retains bounded immutable ZIP bytes until commit or expiry.
type ImportPreview struct {
	ID          uint      `gorm:"primaryKey"`
	OwnerID     uint      `gorm:"not null;index"`
	RevisionID  uint      `gorm:"not null;index"`
	ArtifactID  uint      `gorm:"not null"`
	ExpiresAt   time.Time `gorm:"not null;index"`
	CommittedAt *time.Time
	Owner       User            `gorm:"constraint:OnDelete:CASCADE;"`
	Revision    ProblemRevision `gorm:"constraint:OnDelete:CASCADE;"`
	Artifact    Artifact        `gorm:"constraint:OnDelete:RESTRICT;"`
}

type Artifact struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	OwnerID     uint      `gorm:"not null;index" json:"owner_id"`
	StorageKey  string    `gorm:"uniqueIndex;not null" json:"-"`
	SHA256      string    `gorm:"size:64;index;not null" json:"sha256"`
	SizeBytes   int64     `gorm:"not null;check:size_bytes >= 0" json:"size_bytes"`
	ContentType string    `gorm:"not null" json:"content_type"`
	Filename    string    `gorm:"not null" json:"filename"`
	CreatedAt   time.Time `json:"created_at"`
	Owner       User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
}

type Problem struct {
	ID        uint              `gorm:"primaryKey" json:"id"`
	OwnerID   uint              `gorm:"not null;index" json:"owner_id"`
	Title     string            `gorm:"not null;check:title <> ''" json:"title"`
	Code      string            `gorm:"not null;uniqueIndex;check:code <> ''" json:"code"`
	Kind      string            `gorm:"not null;default:programming;check:kind = 'programming'" json:"kind"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Owner     User              `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Revisions []ProblemRevision `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

type ProblemRevision struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	ProblemID        uint       `gorm:"not null;uniqueIndex:idx_revision_number,priority:1" json:"problem_id"`
	Number           int        `gorm:"not null;check:number > 0;uniqueIndex:idx_revision_number,priority:2" json:"number"`
	State            string     `gorm:"not null;default:draft;check:state IN ('draft','published');index" json:"state"`
	Statement        string     `gorm:"not null" json:"statement"`
	PDFArtifactID    *uint      `gorm:"index" json:"pdf_artifact_id"`
	TimeLimitMillis  *int64     `gorm:"check:time_limit_millis IS NULL OR time_limit_millis > 0" json:"time_limit_millis"`
	MemoryLimitKiB   *int64     `gorm:"column:memory_limit_kib;check:memory_limit_kib IS NULL OR memory_limit_kib > 0" json:"memory_limit_kib"`
	LanguageMetadata string     `gorm:"not null;default:'[]'" json:"language_metadata"`
	CheckerMetadata  string     `gorm:"not null;default:''" json:"checker_metadata"`
	PublishedAt      *time.Time `json:"published_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Problem          Problem    `json:"-"`
	PDFArtifact      *Artifact  `gorm:"foreignKey:PDFArtifactID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Tests            []TestCase `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

type SeriesProblem struct {
	ID                  uint            `gorm:"primaryKey" json:"id"`
	SeriesID            uint            `gorm:"not null;uniqueIndex:idx_series_problem_position,priority:1;uniqueIndex:idx_series_problem_code,priority:1" json:"series_id"`
	ProblemID           uint            `gorm:"not null;index" json:"problem_id"`
	PublishedRevisionID uint            `gorm:"not null" json:"revision_id"`
	Code                string          `gorm:"not null;check:code <> '';uniqueIndex:idx_series_problem_code,priority:2" json:"code"`
	Position            int             `gorm:"not null;check:position >= 0;uniqueIndex:idx_series_problem_position,priority:2" json:"position"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	Series              Series          `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Problem             Problem         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	PublishedRevision   ProblemRevision `gorm:"foreignKey:PublishedRevisionID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
}

type TestCase struct {
	ID                      uint            `gorm:"primaryKey" json:"id"`
	ProblemRevisionID       uint            `gorm:"not null;uniqueIndex:idx_test_revision_position,priority:1" json:"revision_id"`
	Position                int             `gorm:"not null;check:position >= 0;uniqueIndex:idx_test_revision_position,priority:2" json:"position"`
	IsSample                bool            `gorm:"not null;default:false" json:"is_sample"`
	Points                  *int64          `gorm:"check:points IS NULL OR points >= 0" json:"points"`
	TimeLimitOverrideMillis *int64          `gorm:"check:time_limit_override_millis IS NULL OR time_limit_override_millis > 0" json:"time_limit_override_millis"`
	InputArtifactID         uint            `gorm:"not null" json:"input_artifact_id"`
	OutputArtifactID        uint            `gorm:"not null" json:"output_artifact_id"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
	Revision                ProblemRevision `gorm:"foreignKey:ProblemRevisionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	InputArtifact           Artifact        `gorm:"foreignKey:InputArtifactID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	OutputArtifact          Artifact        `gorm:"foreignKey:OutputArtifactID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
}

const (
	RoleAdmin         = "admin"
	RoleUser          = "user"
	ContestDraft      = "draft"
	ContestPublished  = "published"
	RevisionDraft     = "draft"
	RevisionPublished = "published"
)

func (u *User) SetPassword(p string) error {
	password_hash, err := auth.HashPassword(p)
	if err != nil {
		return err
	}
	u.PasswordHash = password_hash
	return nil
}

func (u *User) CheckPassword(p string) bool {
	return auth.CheckPassword(p, u.PasswordHash)
}
